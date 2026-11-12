package shell

import (
	"io"
	"math"
	"runtime/debug"
	"testing"
	"testing/fstest"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestAvailableMemoryFixture(t *testing.T) {
	gunit.Run(new(AvailableMemoryFixture), t)
}

type AvailableMemoryFixture struct {
	*gunit.Fixture
	root fstest.MapFS
}

const mebibyte = 1 << 20

func (this *AvailableMemoryFixture) Setup() {
	this.root = fstest.MapFS{}
}

func (this *AvailableMemoryFixture) file(name, contents string) {
	this.root[name] = &fstest.MapFile{Data: []byte(contents)}
}

func (this *AvailableMemoryFixture) TestNothingKnownMeansUnknown() {
	this.So(availableMemory(this.root, 0), should.Equal, 0)
}

func (this *AvailableMemoryFixture) TestHostMemAvailableIsUsedWithoutCgroupLimits() {
	this.file("proc/meminfo", "MemTotal:       16384000 kB\nMemAvailable:    2048000 kB\n")

	this.So(availableMemory(this.root, 0), should.Equal, 2048000*1024)
}

func (this *AvailableMemoryFixture) TestCgroupV2LimitMinusWorkingSetWinsOverHost() {
	this.file("proc/meminfo", "MemAvailable:   16384000 kB\n")
	this.file("proc/self/cgroup", "0::/\n")
	this.file("sys/fs/cgroup/memory.max", "1073741824\n")
	this.file("sys/fs/cgroup/memory.current", "629145600\n")                    // 600MB used...
	this.file("sys/fs/cgroup/memory.stat", "anon 1\ninactive_file 209715200\n") // ...of which 200MB is reclaimable cache

	this.So(availableMemory(this.root, 0), should.Equal, 1024*mebibyte-400*mebibyte)
}

func (this *AvailableMemoryFixture) TestCgroupV2UnlimitedIsIgnored() {
	this.file("proc/meminfo", "MemAvailable:    1024000 kB\n")
	this.file("proc/self/cgroup", "0::/\n")
	this.file("sys/fs/cgroup/memory.max", "max\n")
	this.file("sys/fs/cgroup/memory.current", "1\n")

	this.So(availableMemory(this.root, 0), should.Equal, 1024000*1024)
}

func (this *AvailableMemoryFixture) TestCgroupV2MemoryHighBelowMaxIsTheLimit() {
	this.file("proc/self/cgroup", "0::/\n")
	this.file("sys/fs/cgroup/memory.max", "8589934592\n")  // 8GB OOM limit...
	this.file("sys/fs/cgroup/memory.high", "2147483648\n") // ...but throttled above 2GB
	this.file("sys/fs/cgroup/memory.current", "536870912\n")

	this.So(availableMemory(this.root, 0), should.Equal, 2048*mebibyte-512*mebibyte)
}

func (this *AvailableMemoryFixture) TestCgroupV2MemoryHighAloneIsALimit() {
	this.file("proc/self/cgroup", "0::/\n")
	this.file("sys/fs/cgroup/memory.max", "max\n")
	this.file("sys/fs/cgroup/memory.high", "1073741824\n")
	this.file("sys/fs/cgroup/memory.current", "104857600\n")

	this.So(availableMemory(this.root, 0), should.Equal, 1024*mebibyte-100*mebibyte)
}

func (this *AvailableMemoryFixture) TestCgroupV2MemoryMaxBelowHighIsTheLimit() {
	this.file("proc/self/cgroup", "0::/\n")
	this.file("sys/fs/cgroup/memory.max", "1073741824\n")
	this.file("sys/fs/cgroup/memory.high", "max\n")
	this.file("sys/fs/cgroup/memory.current", "0\n")

	this.So(availableMemory(this.root, 0), should.Equal, 1024*mebibyte)
}

func (this *AvailableMemoryFixture) TestCgroupV2TightestAncestorLimitApplies() {
	this.file("proc/self/cgroup", "0::/ci.slice/job.scope\n")
	this.file("sys/fs/cgroup/ci.slice/job.scope/memory.max", "max\n")
	this.file("sys/fs/cgroup/ci.slice/memory.max", "536870912\n")
	this.file("sys/fs/cgroup/ci.slice/memory.current", "104857600\n")

	this.So(availableMemory(this.root, 0), should.Equal, 512*mebibyte-100*mebibyte)
}

func (this *AvailableMemoryFixture) TestCgroupV1Limit() {
	this.file("proc/self/cgroup", "12:cpu,cpuacct:/\n4:memory:/docker/abc\n")
	this.file("sys/fs/cgroup/memory/docker/abc/memory.limit_in_bytes", "268435456\n")
	this.file("sys/fs/cgroup/memory/docker/abc/memory.usage_in_bytes", "67108864\n")
	this.file("sys/fs/cgroup/memory/docker/abc/memory.stat", "total_inactive_file 0\n")
	this.file("sys/fs/cgroup/memory/docker/memory.limit_in_bytes", "9223372036854771712\n")

	this.So(availableMemory(this.root, 0), should.Equal, 256*mebibyte-64*mebibyte)
}

func (this *AvailableMemoryFixture) TestExhaustedLimitStillReportsAKnownValue() {
	this.file("proc/self/cgroup", "0::/\n")
	this.file("sys/fs/cgroup/memory.max", "100\n")
	this.file("sys/fs/cgroup/memory.current", "200\n")

	this.So(availableMemory(this.root, 0), should.Equal, 1)
}

func (this *AvailableMemoryFixture) TestGoMemoryLimitHeadroomCanBeTightest() {
	this.file("proc/meminfo", "MemAvailable:   16384000 kB\n")

	this.So(availableMemory(this.root, 300*mebibyte), should.Equal, 300*mebibyte)
}

////////////////////////////////////////////////////////////////////////////////

func TestParallelZstdConcurrencyFixture(t *testing.T) {
	gunit.Run(new(ParallelZstdConcurrencyFixture), t)
}

type ParallelZstdConcurrencyFixture struct {
	*gunit.Fixture
}

func (this *ParallelZstdConcurrencyFixture) TestUnknownMemoryUsesEveryCore() {
	this.So(ParallelZstdConcurrency(16, zstd.SpeedBestCompression, DefaultParallelZstdChunkSize, 0), should.Equal, 16)
}

func (this *ParallelZstdConcurrencyFixture) TestPlentifulMemoryUsesEveryCore() {
	this.So(ParallelZstdConcurrency(16, zstd.SpeedBestCompression, DefaultParallelZstdChunkSize, 8<<30), should.Equal, 16)
}

func (this *ParallelZstdConcurrencyFixture) TestConstrainedMemoryReducesWorkers() {
	// 512MB available → 256MB budget; 64MB fixed; 64MB per best-level worker.
	this.So(ParallelZstdConcurrency(16, zstd.SpeedBestCompression, DefaultParallelZstdChunkSize, 512*mebibyte), should.Equal, 3)
	// Cheaper levels fit more workers into the same memory.
	this.So(ParallelZstdConcurrency(16, zstd.SpeedFastest, DefaultParallelZstdChunkSize, 512*mebibyte), should.Equal, 9)
}

func (this *ParallelZstdConcurrencyFixture) TestTinyMemoryStillUsesOneWorker() {
	this.So(ParallelZstdConcurrency(16, zstd.SpeedBestCompression, DefaultParallelZstdChunkSize, 1), should.Equal, 1)
	this.So(ParallelZstdConcurrency(0, zstd.SpeedFastest, DefaultParallelZstdChunkSize, 0), should.Equal, 1)
}

////////////////////////////////////////////////////////////////////////////////

// GoMemoryLimitFixture changes the process-wide GOMEMLIMIT, so it must not run
// alongside other tests.
func TestGoMemoryLimitFixture(t *testing.T) {
	gunit.Run(new(GoMemoryLimitFixture), t, gunit.Options.AllSequential())
}

type GoMemoryLimitFixture struct {
	*gunit.Fixture
	original int64
}

func (this *GoMemoryLimitFixture) Setup() {
	this.original = debug.SetMemoryLimit(-1)
}
func (this *GoMemoryLimitFixture) Teardown() {
	debug.SetMemoryLimit(this.original)
}

func (this *GoMemoryLimitFixture) TestNoLimitMeansUnknown() {
	debug.SetMemoryLimit(math.MaxInt64)

	this.So(goMemoryHeadroom(), should.Equal, 0)
}

func (this *GoMemoryLimitFixture) TestHeadroomIsLimitMinusRuntimeUsage() {
	const limit = 64 << 30
	debug.SetMemoryLimit(limit)

	headroom := goMemoryHeadroom()

	this.So(headroom, should.BeGreaterThan, 0)
	this.So(headroom, should.BeLessThan, uint64(limit))
	this.So(headroom, should.BeGreaterThan, uint64(limit)-(1<<30)) // a test binary uses far less than 1GB
}

func (this *GoMemoryLimitFixture) TestWriterUsesOneWorkerWhenGoMemoryLimitIsExhausted() {
	debug.SetMemoryLimit(1) // far below what the runtime already holds

	writer, err := NewParallelZstdWriter(io.Discard, zstd.SpeedBestCompression, DefaultParallelZstdChunkSize)
	this.So(err, should.BeNil)
	defer func() { _ = writer.Close() }()

	this.So(writer.concurrency, should.Equal, 1)
}
