// Temporary diagnostic for issue investigation. Not intended for submission.

package fipstest

import (
	entropy "crypto/internal/entropy/v1.0.0"
	"crypto/sha256"
	"flag"
	"fmt"
	"internal/syscall/windows"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
)

var entropyDiagnosticOutput = flag.String("entropy-diagnostic-output", "", "write diagnostic entropy bytes to this file")

func TestEntropyDiagnosticQPC(t *testing.T) {
	const n = 1_000_000
	frequency := windows.QueryPerformanceFrequency()
	deltas := make([]int64, n)
	previous := windows.QueryPerformanceCounter()
	first := previous
	// Keep the timed loop minimal. Analyze the deltas only after collection.
	for i := range deltas {
		now := windows.QueryPerformanceCounter()
		deltas[i] = now - previous
		previous = now
	}
	elapsed := previous - first

	counts := make(map[int64]int)
	var minDelta int64 = math.MaxInt64
	var maxDelta int64 = math.MinInt64
	var zero, negative int
	for _, delta := range deltas {
		counts[delta]++
		if delta < minDelta {
			minDelta = delta
		}
		if delta > maxDelta {
			maxDelta = delta
		}
		if delta == 0 {
			zero++
		} else if delta < 0 {
			negative++
		}
	}
	t.Logf("QPC frequency=%d Hz elapsed=%d ticks mean=%.6f ticks/read min=%d max=%d zero=%d negative=%d distinct=%d",
		frequency, elapsed, float64(elapsed)/n, minDelta, maxDelta, zero, negative, len(counts))
	t.Logf("QPC delta frequencies: %s", formatTopInt64Counts(counts, 20))
}

func TestEntropyDiagnosticSamples(t *testing.T) {
	const n = 1_000_000
	samples := make([]uint8, n)
	frequency := windows.QueryPerformanceFrequency()
	start := windows.QueryPerformanceCounter()
	err := entropy.Samples(samples, &memory)
	end := windows.QueryPerformanceCounter()

	var counts [256]int
	for _, sample := range samples {
		counts[sample]++
	}
	modeValue, modeCount := 0, 0
	distinct := 0
	var shannon float64
	for value, count := range counts {
		if count == 0 {
			continue
		}
		distinct++
		if count > modeCount {
			modeValue, modeCount = value, count
		}
		p := float64(count) / n
		shannon -= p * math.Log2(p)
	}

	longestRun, longestRunValue, longestRunEnd := 0, 0, -1
	currentRun := 0
	var previous uint8
	for i, sample := range samples {
		if i == 0 || sample != previous {
			previous = sample
			currentRun = 1
		} else {
			currentRun++
		}
		if currentRun > longestRun {
			longestRun = currentRun
			longestRunValue = int(sample)
			longestRunEnd = i
		}
	}

	var windowCounts [256]int
	maxWindowCount, maxWindowValue, maxWindowEnd := 0, 0, -1
	for i, sample := range samples {
		windowCounts[sample]++
		if i >= 512 {
			windowCounts[samples[i-512]]--
		}
		if windowCounts[sample] > maxWindowCount {
			maxWindowCount = windowCounts[sample]
			maxWindowValue = int(sample)
			maxWindowEnd = i
		}
	}

	digest := sha256.Sum256(samples)
	elapsedTicks := end - start
	t.Logf("Samples error=%v QPC_frequency=%d elapsed_ticks=%d elapsed_seconds=%.9f ticks_per_sample=%.6f sha256=%x",
		err, frequency, elapsedTicks, float64(elapsedTicks)/float64(frequency), float64(elapsedTicks)/n, digest)
	t.Logf("Distribution distinct=%d mode=%d mode_count=%d mode_proportion=%.9f shannon_bits=%.6f",
		distinct, modeValue, modeCount, float64(modeCount)/n, shannon)
	t.Logf("RCT longest_run=%d value=%d end_index=%d; APT max_window_count=%d value=%d end_index=%d",
		longestRun, longestRunValue, longestRunEnd, maxWindowCount, maxWindowValue, maxWindowEnd)
	t.Logf("Byte histogram: %s", formatByteCounts(counts[:]))

	if *entropyDiagnosticOutput != "" {
		if err := os.WriteFile(*entropyDiagnosticOutput, samples, 0600); err != nil {
			t.Fatalf("write diagnostic output: %v", err)
		}
		t.Logf("Wrote %d samples to %s", len(samples), *entropyDiagnosticOutput)
	}
}

type int64Count struct {
	value int64
	count int
}

func formatTopInt64Counts(counts map[int64]int, limit int) string {
	entries := make([]int64Count, 0, len(counts))
	for value, count := range counts {
		entries = append(entries, int64Count{value, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].value < entries[j].value
	})
	if len(entries) > limit {
		entries = entries[:limit]
	}
	parts := make([]string, len(entries))
	for i, entry := range entries {
		parts[i] = fmt.Sprintf("%d:%d", entry.value, entry.count)
	}
	return strings.Join(parts, ",")
}

func formatByteCounts(counts []int) string {
	parts := make([]string, 0, len(counts))
	for value, count := range counts {
		if count != 0 {
			parts = append(parts, fmt.Sprintf("%d:%d", value, count))
		}
	}
	return strings.Join(parts, ",")
}
