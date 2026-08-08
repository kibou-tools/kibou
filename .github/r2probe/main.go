// Command r2probe measures the filesystem and archive throughput floor for a
// realistic GOCACHE tree, primarily on Windows/NTFS.
//
// Throwaway measurement tool; see .cache/2026-08-07-ci-cache-sizing/ for the
// analysis it feeds. Deliberately stdlib-only: the zstd codec is invoked as a
// subprocess so that -T0 (all cores) is available without a dependency.
//
// Subcommands:
//
//	stat    -dir D                        report entry count and size histogram
//	ceiling -dir D -out O -workers W       create the same tree, no archive
//	pack    -dir D -out O -shards K        tar+zstd, K independent streams
//	unpack  -in O -out D -workers W        extract, W concurrent file writers
//
// ceiling is the important one: it establishes the floor that no extractor can
// beat, because it does nothing but create files.
package main

import (
	"archive/tar"
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
)

func main() {
	log := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
	if len(os.Args) < 2 {
		log("usage: r2probe <stat|ceiling|pack|unpack> [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "stat":
		err = cmdStat(os.Args[2:])
	case "ceiling":
		err = cmdCeiling(os.Args[2:])
	case "pack":
		err = cmdPack(os.Args[2:])
	case "unpack":
		err = cmdUnpack(os.Args[2:])
	default:
		err = fmt.Errorf("unknown subcommand %q", os.Args[1])
	}
	if err != nil {
		log("r2probe: %v", err)
		os.Exit(1)
	}
}

// result prints one measurement in a grep-friendly single-line form.
func result(op string, d time.Duration, files int, bytes int64, extra string) {
	mb := float64(bytes) / (1 << 20)
	fmt.Printf("RESULT\top=%s\t%s\tsecs=%.2f\tfiles=%d\tMB=%.1f\tfiles_per_s=%.0f\tMB_per_s=%.1f\n",
		op, extra, d.Seconds(), files, mb,
		float64(files)/d.Seconds(), mb/d.Seconds())
}

// file is one regular file in the cache tree, path relative to the root.
type file struct {
	rel  string
	size int64
}

// walk collects every regular file under root, plus every directory, so that a
// tree can be recreated exactly. Cache entries may themselves be directories
// (executable cache entries), so this recurses rather than assuming depth 2.
func walk(root string) (files []file, dirs []string, err error) {
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, rel)
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, file{rel: rel, size: info.Size()})
		return nil
	})
	return files, dirs, err
}

func totalSize(files []file) int64 {
	var n int64
	for _, f := range files {
		n += f.size
	}
	return n
}

func cmdStat(args []string) error {
	fs := flag.NewFlagSet("stat", flag.ExitOnError)
	dir := fs.String("dir", "", "cache directory")
	fs.Parse(args)
	if *dir == "" {
		return fmt.Errorf("-dir is required")
	}

	start := time.Now()
	files, dirs, err := walk(*dir)
	if err != nil {
		return err
	}
	elapsed := time.Since(start)

	// Entries are the depth-2 names ending in -a or -d; some are directories.
	var entryFiles, entryDirs int
	roots, err := os.ReadDir(*dir)
	if err != nil {
		return err
	}
	for _, r := range roots {
		if !r.IsDir() || len(r.Name()) != 2 {
			continue
		}
		sub, err := os.ReadDir(filepath.Join(*dir, r.Name()))
		if err != nil {
			return err
		}
		for _, e := range sub {
			if !strings.HasSuffix(e.Name(), "-a") && !strings.HasSuffix(e.Name(), "-d") {
				continue
			}
			if e.IsDir() {
				entryDirs++
			} else {
				entryFiles++
			}
		}
	}

	buckets := []struct {
		label string
		max   int64
	}{
		{"<1KB", 1 << 10}, {"1-16KB", 1 << 14}, {"16-128KB", 1 << 17},
		{"128KB-1MB", 1 << 20}, {"1-16MB", 1 << 24}, {">16MB", 1 << 62},
	}
	counts := make([]int, len(buckets))
	sizes := make([]int64, len(buckets))
	for _, f := range files {
		for i, b := range buckets {
			if f.size < b.max {
				counts[i]++
				sizes[i] += f.size
				break
			}
		}
	}

	total := totalSize(files)
	fmt.Printf("walk of %s took %.2fs\n", *dir, elapsed.Seconds())
	fmt.Printf("regular files=%d dirs=%d totalMB=%.1f\n", len(files), len(dirs), float64(total)/(1<<20))
	fmt.Printf("depth-2 entries: files=%d dirs=%d\n", entryFiles, entryDirs)
	for i, b := range buckets {
		fmt.Printf("  %-12s n=%-7d %5.1f%%  %8.1f MB %5.1f%%\n",
			b.label, counts[i], 100*float64(counts[i])/float64(len(files)),
			float64(sizes[i])/(1<<20), 100*float64(sizes[i])/float64(total))
	}
	return nil
}

// pattern is written instead of the original bytes so that ceiling measures
// pure write cost without reading the source tree. It is not all zeros, to
// avoid any sparse-file or compression shortcut.
//
// It is sized to hold the largest cache entry outright so that writePattern
// issues a single write per file, matching what unpack does via os.WriteFile.
// An earlier 1 MiB buffer wrote multi-megabyte entries in a loop, which made
// the "floor" slower than the extractor it was supposed to bound.
var pattern = func() []byte {
	b := make([]byte, 64<<20)
	for i := range b {
		b[i] = byte(i*31 + 7)
	}
	return b
}()

// writePattern is only ever a reader of pattern, so the shared buffer is safe
// to use concurrently from every worker.
func writePattern(path string, size int64) error {
	if size <= int64(len(pattern)) {
		return os.WriteFile(path, pattern[:size], 0o644)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for size > 0 {
		n := int64(len(pattern))
		if size < n {
			n = size
		}
		if _, err := f.Write(pattern[:n]); err != nil {
			return err
		}
		size -= n
	}
	return nil
}

func cmdCeiling(args []string) error {
	fs := flag.NewFlagSet("ceiling", flag.ExitOnError)
	dir := fs.String("dir", "", "source cache directory (read for its shape only)")
	out := fs.String("out", "", "destination directory to create")
	workers := fs.Int("workers", runtime.NumCPU(), "concurrent file creators")
	empty := fs.Bool("empty", false, "create zero-length files (isolates metadata cost)")
	fs.Parse(args)
	if *dir == "" || *out == "" {
		return fmt.Errorf("-dir and -out are required")
	}

	files, dirs, err := walk(*dir)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(*out); err != nil {
		return err
	}

	// Pre-create directories serially so the measurement covers file creation
	// only, and workers never contend on MkdirAll for the same parent.
	sort.Strings(dirs)
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	for _, d := range dirs {
		if err := os.Mkdir(filepath.Join(*out, d), 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	start := time.Now()
	ch := make(chan file, 1024)
	var wg sync.WaitGroup
	var failed atomic.Int64
	for range *workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range ch {
				size := f.size
				if *empty {
					size = 0
				}
				if err := writePattern(filepath.Join(*out, f.rel), size); err != nil {
					failed.Add(1)
				}
			}
		}()
	}
	for _, f := range files {
		ch <- f
	}
	close(ch)
	wg.Wait()
	elapsed := time.Since(start)

	if n := failed.Load(); n > 0 {
		return fmt.Errorf("%d files failed to write", n)
	}
	written := totalSize(files)
	if *empty {
		written = 0
	}
	result("ceiling", elapsed, len(files), written,
		fmt.Sprintf("workers=%d\tempty=%v", *workers, *empty))
	return nil
}

// shardName is the archive file for shard i.
func shardName(out string, i int) string {
	return filepath.Join(out, fmt.Sprintf("shard-%02d.tzst", i))
}

// zstdThreads returns the -T value for a shard count: with several concurrent
// streams, each gets a slice of the cores rather than all of them.
func zstdThreads(shards int) int {
	n := runtime.NumCPU() / shards
	if n < 1 {
		n = 1
	}
	return n
}

func cmdPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	dir := fs.String("dir", "", "cache directory to pack")
	out := fs.String("out", "", "output directory for shard archives")
	shards := fs.Int("shards", 1, "number of independent tar+zstd streams")
	level := fs.Int("level", 3, "zstd compression level")
	fs.Parse(args)
	if *dir == "" || *out == "" {
		return fmt.Errorf("-dir and -out are required")
	}
	if *shards < 1 {
		return fmt.Errorf("-shards must be >= 1")
	}

	files, _, err := walk(*dir)
	if err != nil {
		return err
	}
	// Assign whole top-level directories to shards so that a shard is
	// independently extractable and no two shards create the same parent.
	groups := make([][]file, *shards)
	for _, f := range files {
		top := f.rel
		if i := strings.IndexAny(top, `/\`); i >= 0 {
			top = top[:i]
		}
		h := 0
		for _, c := range top {
			h = h*31 + int(c)
		}
		g := ((h % *shards) + *shards) % *shards
		groups[g] = append(groups[g], f)
	}

	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	start := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, *shards)
	for i := range *shards {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = packShard(*dir, shardName(*out, i), groups[i], *level, zstdThreads(*shards))
		}(i)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	elapsed := time.Since(start)

	var archived int64
	for i := range *shards {
		info, err := os.Stat(shardName(*out, i))
		if err != nil {
			return err
		}
		archived += info.Size()
	}
	result("pack", elapsed, len(files), totalSize(files),
		fmt.Sprintf("shards=%d\tlevel=%d\tarchiveMB=%.1f", *shards, *level, float64(archived)/(1<<20)))
	return nil
}

func packShard(root, archive string, files []file, level, threads int) error {
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	bw := bufio.NewWriterSize(f, 4<<20)
	zw, err := zstd.NewWriter(bw,
		zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)),
		zstd.WithEncoderConcurrency(threads))
	if err != nil {
		return err
	}

	tw := tar.NewWriter(zw)
	for _, file := range files {
		src, err := os.Open(filepath.Join(root, file.rel))
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:     filepath.ToSlash(file.rel),
			Mode:     0o644,
			Size:     file.size,
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			src.Close()
			return err
		}
		if _, err := io.CopyN(tw, src, file.size); err != nil {
			src.Close()
			return err
		}
		src.Close()
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return bw.Flush()
}

func cmdUnpack(args []string) error {
	fs := flag.NewFlagSet("unpack", flag.ExitOnError)
	in := fs.String("in", "", "directory containing shard archives")
	out := fs.String("out", "", "destination directory")
	workers := fs.Int("workers", runtime.NumCPU(), "concurrent file writers per shard")
	fs.Parse(args)
	if *in == "" || *out == "" {
		return fmt.Errorf("-in and -out are required")
	}

	shards, err := filepath.Glob(filepath.Join(*in, "shard-*.tzst"))
	if err != nil {
		return err
	}
	if len(shards) == 0 {
		return fmt.Errorf("no shard archives in %s", *in)
	}
	sort.Strings(shards)
	if err := os.RemoveAll(*out); err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}

	start := time.Now()
	var wg sync.WaitGroup
	errs := make([]error, len(shards))
	var count atomic.Int64
	var bytes atomic.Int64
	for i, s := range shards {
		wg.Add(1)
		go func(i int, s string) {
			defer wg.Done()
			errs[i] = unpackShard(s, *out, *workers, &count, &bytes)
		}(i, s)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	elapsed := time.Since(start)
	result("unpack", elapsed, int(count.Load()), bytes.Load(),
		fmt.Sprintf("shards=%d\tworkers=%d", len(shards), *workers))
	return nil
}

// unpackShard streams one archive, handing file bodies to a pool of writers so
// that tar parsing (serial by nature) does not gate filesystem throughput.
func unpackShard(archive, out string, workers int, count, bytes *atomic.Int64) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	zr, err := zstd.NewReader(bufio.NewReaderSize(f, 4<<20),
		zstd.WithDecoderConcurrency(zstdThreads(1)))
	if err != nil {
		return err
	}
	defer zr.Close()

	type job struct {
		path string
		data []byte
	}
	ch := make(chan job, workers*4)
	var wg sync.WaitGroup
	var failed atomic.Int64
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				if err := os.WriteFile(j.path, j.data, 0o644); err != nil {
					failed.Add(1)
				}
			}
		}()
	}

	// Parent directories are created by the reader, serially: MkdirAll from
	// many goroutines on a shared path is both racy-looking and slower.
	seen := make(map[string]bool)
	tr := tar.NewReader(zr)
	readErr := func() error {
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if hdr.Typeflag != tar.TypeReg {
				continue
			}
			path := filepath.Join(out, filepath.FromSlash(hdr.Name))
			if parent := filepath.Dir(path); !seen[parent] {
				if err := os.MkdirAll(parent, 0o755); err != nil {
					return err
				}
				seen[parent] = true
			}
			data := make([]byte, hdr.Size)
			if _, err := io.ReadFull(tr, data); err != nil {
				return err
			}
			count.Add(1)
			bytes.Add(hdr.Size)
			ch <- job{path: path, data: data}
		}
	}()
	close(ch)
	wg.Wait()
	if n := failed.Load(); n > 0 && readErr == nil {
		readErr = fmt.Errorf("%d files failed to write", n)
	}
	return readErr
}
