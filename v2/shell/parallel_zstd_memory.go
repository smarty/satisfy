package shell

import "github.com/klauspost/compress/zstd"

// ParallelZstdConcurrency returns how many compression workers fit in the
// available memory. It returns cores unchanged when availableMemory is 0
// (unknown) or large enough for every worker, so machines with room to spare
// are never slowed down; it only cuts the worker count (never below 1) when
// the full count could get the process OOM-killed.
func ParallelZstdConcurrency(cores int, level zstd.EncoderLevel, chunkSize int, availableMemory uint64) int {
	cores = max(cores, 1)
	if availableMemory == 0 {
		return cores
	}
	// Only plan to use half of what is available: the rest covers the tar/hash
	// pipeline, uncollected garbage between GC cycles, and other processes.
	budget := availableMemory / 2
	fixed, perWorker := parallelZstdMemoryCost(level, chunkSize)
	if budget <= fixed+perWorker {
		return 1
	}
	return int(min(uint64(cores), (budget-fixed)/perWorker))
}

// parallelZstdMemoryCost estimates the heap the writer holds no matter how
// many workers it has (the chunk being filled, the frame being written out,
// and buffers waiting in the pools or for GC) and the heap each additional
// worker adds (a queued input chunk, its encoded output, and that worker's
// encoder state).
//
// These figures come from measuring peak HeapInuse while compressing 64MB of
// mixed text and random bytes with 8MB chunks, comparing 1 and 8 workers:
// about 64–80MB fixed, plus per worker roughly 18MB (fastest), 27MB (default),
// 30MB (better) and 60MB (best).
func parallelZstdMemoryCost(level zstd.EncoderLevel, chunkSize int) (fixed, perWorker uint64) {
	chunk := uint64(max(chunkSize, 0))
	return 8 * chunk, 2*chunk + parallelZstdEncoderState(level)
}

// parallelZstdEncoderState is the measured history and match-table memory one
// klauspost zstd encoder holds for each concurrent EncodeAll call.
func parallelZstdEncoderState(level zstd.EncoderLevel) uint64 {
	switch {
	case level <= zstd.SpeedFastest:
		return 4 << 20
	case level <= zstd.SpeedDefault:
		return 12 << 20
	case level <= zstd.SpeedBetterCompression:
		return 16 << 20
	default:
		return 48 << 20
	}
}
