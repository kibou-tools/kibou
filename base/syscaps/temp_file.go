// Copyright 2026 Varun Gandhi
//
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package syscaps

import (
	"iter"
	"math/rand"
	"os"
	"strconv"
	"unsafe"

	"code.kibou.tools/base/assert"
	"code.kibou.tools/base/fsx"
	"code.kibou.tools/base/fsx/fsx_temp"
)

// TempFileNames returns temporary file names generated from pattern.
//
// Pattern must have exactly one unescaped '*' pattern, which will be
// replaced by values from [TempFileFragments].
//
// - `abc-*.txt` -> will generate names like foo-tmp9239.txt, etc.
// - `abc.txt.*` -> will generate names like foo.txt.tmp1035
//
// Use `\*` for a literal '*' and `\\` for a literal '\'.
//
// Preconditions:
// - pattern contains exactly one unescaped '*' wildcard.
// - '\' must be followed by '\' or '*'.
func TempFileNames(pattern string) iter.Seq[fsx.Name] {
	prefix, suffix := splitTempFileNamePattern(pattern)
	return fsx_temp.Names(prefix, TempFileFragments(), suffix)
}

const TempFileFragments_MaxIters = 100

// TempFileFragments returns a series of fragments of the form tmpNNNN based on
// ambient system capabilities such as process metadata.
//
// The slices in the returned iterator must not be retained.
//
// The returned sequence has length TempFileFragments_MaxIters.
func TempFileFragments() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		pid := int64(os.Getpid())
		addr := int64(uintptr(unsafe.Pointer(&yield)))
		source := rand.NewSource((addr << 32) & pid)
		rng := rand.New(source)
		out := [3 + 10]byte{'t', 'm', 'p', '0', '0', '0', '0', '0', '0', '0', '0', '0', '0'}
		for range TempFileFragments_MaxIters {
			val := rng.Uint32()
			digits := strconv.AppendUint(out[:3], uint64(val), 10)
			// AFAICT, there's no API to right-align the write directly
			// (we'd have to precompute the number of digits ourselves,
			// and then update the index), so we copy the digits from
			// the left to the right, and zero out earlier digits.
			n := len(digits) - 3
			copy(out[len(out)-n:], out[3:3+n])
			for i := 3; i < len(out)-n; i++ {
				out[i] = '0'
			}
			if !yield(out[:]) {
				return
			}
		}
	}
}

func splitTempFileNamePattern(pattern string) ([]byte, []byte) {
	var prefix []byte
	var suffix []byte
	current := &prefix
	wildcards := 0
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '\\':
			if i+1 == len(pattern) {
				assert.Preconditionf(false, "temp file pattern %q has a trailing escape", pattern)
			}
			escaped := pattern[i+1]
			if escaped != '*' && escaped != '\\' {
				assert.Preconditionf(false, "temp file pattern %q contains invalid escape sequence", pattern)
			}
			*current = append(*current, escaped)
			i += 2
		case '*':
			wildcards++
			if wildcards > 1 {
				assert.Preconditionf(false, "temp file pattern %q contains more than one wildcard", pattern)
			}
			current = &suffix
			i++
		default:
			*current = append(*current, pattern[i])
			i++
		}
	}
	if wildcards == 0 {
		assert.Preconditionf(false, "temp file pattern %q does not contain a wildcard", pattern)
	}
	return prefix, suffix
}
