package shell

import (
	"bufio"
	"bytes"
	"io/fs"
	"math"
	"os"
	"path"
	"runtime/debug"
	"runtime/metrics"
	"strconv"
	"strings"
)

// AvailableMemory estimates how many more bytes this process can allocate
// before it risks being OOM-killed or blowing through GOMEMLIMIT. It returns 0
// when nothing is known (e.g. on macOS, which pages to swap instead of
// OOM-killing); callers treat 0 as "no limit".
func AvailableMemory() uint64 {
	return availableMemory(os.DirFS("/"), goMemoryHeadroom())
}

// availableMemory takes the tightest of every limit it can find: the Go
// runtime's soft limit, each enclosing cgroup's hard limit (v2 or v1), and the
// host's MemAvailable. A zero argument or missing file means "unknown".
func availableMemory(root fs.FS, goHeadroom uint64) uint64 {
	available := uint64(math.MaxUint64)
	for _, candidate := range []uint64{
		goHeadroom,
		cgroupV2Headroom(root),
		cgroupV1Headroom(root),
		meminfoAvailable(root),
	} {
		if candidate > 0 {
			available = min(available, candidate)
		}
	}
	if available == math.MaxUint64 {
		return 0
	}
	return available
}

// goMemoryHeadroom is GOMEMLIMIT minus what the runtime already holds, or 0
// when GOMEMLIMIT is not set.
func goMemoryHeadroom() uint64 {
	limit := debug.SetMemoryLimit(-1) // a negative input only reads the limit
	if limit <= 0 || limit == math.MaxInt64 {
		return 0
	}
	sample := []metrics.Sample{{Name: "/memory/classes/total:bytes"}}
	metrics.Read(sample)
	return headroom(uint64(limit), sample[0].Value.Uint64())
}

// cgroupV2Headroom walks from this process's cgroup up to the root (limits on
// an enclosing slice apply too) and returns the smallest memory.max minus
// working set. Working set excludes inactive page cache, which the kernel
// reclaims before OOM-killing (the same measure Kubernetes uses for eviction).
func cgroupV2Headroom(root fs.FS) uint64 {
	group, found := ownCgroup(root, func(hierarchy, controllers string) bool { return hierarchy == "0" && controllers == "" })
	if !found {
		return 0
	}
	smallest := uint64(0)
	for directory := range cgroupAncestors("sys/fs/cgroup", group) {
		limit, ok := readCgroupNumber(root, path.Join(directory, "memory.max"))
		if !ok {
			continue
		}
		usage, _ := readCgroupNumber(root, path.Join(directory, "memory.current"))
		inactive := readCgroupStat(root, path.Join(directory, "memory.stat"), "inactive_file")
		smallest = tighter(smallest, headroom(limit, workingSet(usage, inactive)))
	}
	return smallest
}

// cgroupV1Headroom is cgroupV2Headroom for the legacy memory controller.
// Unlimited v1 groups report a huge page-aligned limit, which never wins the
// min() against real limits.
func cgroupV1Headroom(root fs.FS) uint64 {
	group, found := ownCgroup(root, func(_, controllers string) bool {
		return containsField(controllers, "memory")
	})
	if !found {
		return 0
	}
	smallest := uint64(0)
	for directory := range cgroupAncestors("sys/fs/cgroup/memory", group) {
		limit, ok := readCgroupNumber(root, path.Join(directory, "memory.limit_in_bytes"))
		if !ok {
			continue
		}
		usage, _ := readCgroupNumber(root, path.Join(directory, "memory.usage_in_bytes"))
		inactive := readCgroupStat(root, path.Join(directory, "memory.stat"), "total_inactive_file")
		smallest = tighter(smallest, headroom(limit, workingSet(usage, inactive)))
	}
	return smallest
}

// ownCgroup finds this process's cgroup path in /proc/self/cgroup, whose lines
// look like "hierarchy-ID:controller-list:path". The path is "/" inside most
// containers because of cgroup namespaces.
func ownCgroup(root fs.FS, matches func(hierarchy, controllers string) bool) (string, bool) {
	raw, err := fs.ReadFile(root, "proc/self/cgroup")
	if err != nil {
		return "", false
	}
	for line := range strings.Lines(string(raw)) {
		fields := strings.SplitN(strings.TrimSpace(line), ":", 3)
		if len(fields) == 3 && matches(fields[0], fields[1]) {
			return fields[2], true
		}
	}
	return "", false
}

// cgroupAncestors yields mount/group, then each parent directory up to mount.
func cgroupAncestors(mount, group string) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for current := path.Clean("/" + group); ; current = path.Dir(current) {
			if !yield(path.Join(mount, current)) || current == "/" {
				return
			}
		}
	}
}

func readCgroupNumber(root fs.FS, name string) (uint64, bool) {
	raw, err := fs.ReadFile(root, name)
	if err != nil {
		return 0, false
	}
	value, err := strconv.ParseUint(string(bytes.TrimSpace(raw)), 10, 64)
	return value, err == nil // "max" (unlimited) fails to parse and is skipped
}

func readCgroupStat(root fs.FS, name, key string) uint64 {
	value, _ := readKeyedValue(root, name, key)
	return value
}

// meminfoAvailable is the host's MemAvailable, the kernel's own estimate of
// memory that can be allocated without swapping.
func meminfoAvailable(root fs.FS) uint64 {
	kilobytes, _ := readKeyedValue(root, "proc/meminfo", "MemAvailable:")
	return kilobytes * 1024
}

// readKeyedValue finds a "key value [unit]" line and parses its value.
func readKeyedValue(root fs.FS, name, key string) (uint64, bool) {
	file, err := root.Open(name)
	if err != nil {
		return 0, false
	}
	defer func() { _ = file.Close() }()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == key {
			value, err := strconv.ParseUint(fields[1], 10, 64)
			return value, err == nil
		}
	}
	return 0, false
}

func containsField(list, field string) bool {
	for item := range strings.SplitSeq(list, ",") {
		if item == field {
			return true
		}
	}
	return false
}

func workingSet(usage, inactiveFile uint64) uint64 {
	if inactiveFile >= usage {
		return 0
	}
	return usage - inactiveFile
}

// headroom never returns 0 for an exhausted limit, because 0 means "unknown"
// to callers; 1 byte still forces the minimum worker count.
func headroom(limit, used uint64) uint64 {
	if used >= limit {
		return 1
	}
	return limit - used
}

func tighter(current, candidate uint64) uint64 {
	if current == 0 {
		return candidate
	}
	return min(current, candidate)
}
