package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2"
)

func TestRunFixture(t *testing.T) {
	gunit.Run(new(RunFixture), t)
}

type RunFixture struct {
	*gunit.Fixture
	stdin  *bytes.Buffer
	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func (this *RunFixture) Setup() {
	this.stdin = new(bytes.Buffer)
	this.stdout = new(bytes.Buffer)
	this.stderr = new(bytes.Buffer)
}

func (this *RunFixture) run(args ...string) int {
	return run(context.Background(), args, this.stdin, this.stdout, this.stderr)
}

func (this *RunFixture) TestVersionIsPrintedToStdout() {
	this.So(this.run("version"), should.Equal, exitSuccess)
	this.So(this.stdout.String(), should.StartWith, "satisfy [")
}

func (this *RunFixture) TestExplicitDownloadSubcommandIsRejected() {
	this.So(this.run("download"), should.Equal, exitFailure)
	this.So(this.stderr.String(), should.ContainSubstring, "no need to supply 'download'")
}

func (this *RunFixture) TestHelpExitsSuccessfully() {
	this.So(this.run("-h"), should.Equal, exitSuccess)
	this.So(this.stderr.String(), should.ContainSubstring, "Usage of satisfy")
}

func (this *RunFixture) TestTagsWithoutSubcommandIsAUsageError() {
	this.So(this.run("tags"), should.Equal, exitFailure)
	this.So(this.stderr.String(), should.ContainSubstring, "usage: satisfy tags")
}

func (this *RunFixture) TestInvalidFlagIsAFailure() {
	this.So(this.run("-no-such-flag"), should.Equal, exitFailure)
}

func (this *RunFixture) TestEmptyDependencyListingFromStdinSucceedsWithoutCredentials() {
	this.stdin.WriteString(`{"dependencies": []}`)
	this.So(this.run(), should.Equal, exitSuccess)
	this.So(this.stderr.String(), should.ContainSubstring, "No dependencies provided")
	this.So(this.stdout.String(), should.BeEmpty)
}

func (this *RunFixture) TestMissingDependencyFileShowsAnExample() {
	this.So(this.run("-json", "/no/such/file.json"), should.Equal, exitFailure)
	this.So(this.stderr.String(), should.ContainSubstring, "Example json file")
}

func (this *RunFixture) TestLatestRequiresABucket() {
	this.So(this.run("latest", "-package", "p"), should.Equal, exitFailure)
	this.So(this.stderr.String(), should.ContainSubstring, "-bucket is required")
}

func TestExitCodeFixture(t *testing.T) {
	gunit.Run(new(ExitCodeFixture), t)
}

type ExitCodeFixture struct {
	*gunit.Fixture
	env    environment
	stderr *bytes.Buffer
}

func (this *ExitCodeFixture) Setup() {
	this.stderr = new(bytes.Buffer)
	this.env = environment{stderr: this.stderr, logger: newTestLogger(this.stderr)}
}

func (this *ExitCodeFixture) TestSuccess() {
	this.So(exitCode(this.env, nil), should.Equal, exitSuccess)
}

func (this *ExitCodeFixture) TestPackageExistsIsExitCodeTwo() {
	err := fmt.Errorf("wrapped: %w", satisfy.ErrPackageExists)
	this.So(exitCode(this.env, err), should.Equal, exitPackageExists)
}

func (this *ExitCodeFixture) TestOtherErrorsAreExitCodeOneAndLogged() {
	this.So(exitCode(this.env, errors.New("boom")), should.Equal, exitFailure)
	this.So(this.stderr.String(), should.ContainSubstring, "boom")
}

func TestRenderProgressFixture(t *testing.T) {
	gunit.Run(new(RenderProgressFixture), t)
}

type RenderProgressFixture struct {
	*gunit.Fixture
}

func (this *RenderProgressFixture) TestInProgressRedrawsTheLine() {
	rendered := renderProgress(satisfy.Progress{
		Phase: satisfy.ProgressExtracting, Name: "file.txt", Written: 1024, Total: 2048,
	})
	this.So(rendered, should.Equal, "\033[2K\rExtracting file.txt: 1 KB of 2 KB.")
}

func (this *RenderProgressFixture) TestDoneEndsTheLine() {
	rendered := renderProgress(satisfy.Progress{
		Phase: satisfy.ProgressArchiving, Name: "file.txt", Written: 1024, Total: 1024, Done: true,
	})
	this.So(strings.HasSuffix(rendered, "\n"), should.BeTrue)
	this.So(rendered, should.ContainSubstring, "Done archiving file.txt (1 KB)")
}
