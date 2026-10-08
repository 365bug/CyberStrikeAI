package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGo is a scripted stand-in for the Go toolchain: `go env GOVERSION` answers, and
// `go build -o <out> ...` either writes a recognisable file or fails, decided by a marker
// file the test toggles. It goes first on PATH (the real PATH stays behind it, because git
// still has to resolve), so the update's real build step runs without compiling the whole
// server inside a test.
type fakeGo struct {
	marker string
}

func scriptFakeGo(t *testing.T) *fakeGo {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "fail-build")
	f := &fakeGo{marker: marker}
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = env ]; then echo go1.23.4; exit 0; fi\n" +
		"out=\"\"\n" +
		"prev=\"\"\n" +
		"for a in \"$@\"; do\n" +
		"  if [ \"$prev\" = -o ]; then out=\"$a\"; fi\n" +
		"  prev=\"$a\"\n" +
		"done\n" +
		"if [ -f \"" + marker + "\" ]; then echo 'vet: internal/foo.go:12: fake failure' >&2; exit 1; fi\n" +
		"echo 'fake binary' > \"$out\"\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return f
}

func (f *fakeGo) failBuild(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(f.marker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *fakeGo) succeed(t *testing.T) {
	t.Helper()
	if err := os.Remove(f.marker); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// hideToolchain takes `go` off the PATH child processes see while leaving everything else
// (git) reachable: a directory holding a symlink to the real git and nothing else.
func hideToolchain(t *testing.T) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if err := os.Symlink(realGit, filepath.Join(dir, "git")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// buildOpts drives an update with a real binary to swap - the configuration of every
// installed tree, where "the source moved but the binary did not" is even possible.
func buildOpts(tr *tree) Options {
	return Options{Root: tr.install, Repo: tr.upstream, BinaryName: Binary}
}

// TestRetryAfterFailedBuildCompilesTheBinary: a failed build leaves the source updated
// (HEAD is on the new commit), so a retry sees "nothing to pull" - and used to report
// success while the old binary stayed in place. The retry has to finish the build.
func TestRetryAfterFailedBuildCompilesTheBinary(t *testing.T) {
	requireGit(t)
	tr := newTree(t)
	fake := scriptFakeGo(t)
	fake.failBuild(t)

	bin := filepath.Join(tr.install, Binary)
	writeFile(t, bin, "old binary bytes")
	tr.upstreamCommit(t, "upstream moves", map[string]string{"internal_service.go": "package service\n\nconst Version = \"2\"\n"})

	res, err := Apply(context.Background(), buildOpts(tr), nil)
	ue, ok := err.(*Error)
	if !ok || ue.Reason != "build_failed" {
		t.Fatalf("first apply: err=%v result=%+v, want build_failed", err, res)
	}
	if res.BinaryBuilt {
		t.Error("a failed build must not claim a binary")
	}
	if got := readFile(t, bin); got != "old binary bytes" {
		t.Errorf("binary after the failed build = %q, want the old one untouched", got)
	}
	snap, err := Status(context.Background(), buildOpts(tr))
	if err != nil {
		t.Fatal(err)
	}
	if !snap.BuildPending {
		t.Error("after a failed build the tree owes a compile; Status must say so")
	}

	// Same tree, a working toolchain: the retry compiles instead of reporting success for
	// work that never happened.
	fake.succeed(t)
	res, err = Apply(context.Background(), buildOpts(tr), nil)
	if err != nil {
		t.Fatalf("retry failed: %v (result %+v)", err, res)
	}
	if !res.BinaryBuilt || res.Commits != 0 || res.FromCommit != res.ToCommit {
		t.Errorf("retry result = %+v, want a build with no commits moved", res)
	}
	if got := readFile(t, bin); !strings.Contains(got, "fake binary") {
		t.Errorf("binary after retry = %q, want the freshly built one", got)
	}
	if got := readFile(t, bin+".prev"); got != "old binary bytes" {
		t.Errorf("prev binary = %q, want the one the swap kept", got)
	}
	if buildPending(tr.install) {
		t.Error("a finished build must clear the pending marker")
	}
}

// TestRetryFinishesTheBuildWhenTheStateWriteFailed: the merge landed, but the update
// aborted before the state file could be written (a full disk, a directory sitting where
// the file goes). The tree still owes a binary, and the retry must know that - the marker
// is written the moment the merge lands, not after the bookkeeping that may fail.
func TestRetryFinishesTheBuildWhenTheStateWriteFailed(t *testing.T) {
	requireGit(t)
	tr := newTree(t)
	fake := scriptFakeGo(t)
	fake.succeed(t)

	bin := filepath.Join(tr.install, Binary)
	writeFile(t, bin, "old binary bytes")
	tr.upstreamCommit(t, "upstream moves", map[string]string{"internal_service.go": "package service\n\nconst Version = \"2\"\n"})

	// A directory where writeState's temp file goes makes the state write fail.
	blocked := filepath.Join(tr.install, ".update-state.json.tmp")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Apply(context.Background(), buildOpts(tr), nil)
	if ue, ok := err.(*Error); !ok || ue.Reason != "state_unwritable" {
		t.Fatalf("want state_unwritable, got %v (result %+v)", err, res)
	}
	if got := readFile(t, bin); got != "old binary bytes" {
		t.Errorf("no build may happen while the state write is broken: %q", got)
	}

	// Clear the obstruction and retry: the owed build must happen, not a success report.
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	res, err = Apply(context.Background(), buildOpts(tr), nil)
	if err != nil {
		t.Fatalf("retry failed: %v (result %+v)", err, res)
	}
	if !res.BinaryBuilt {
		t.Fatalf("retry result = %+v, want the owed build", res)
	}
	if got := readFile(t, bin); !strings.Contains(got, "fake binary") {
		t.Errorf("binary after retry = %q, want the compiled one", got)
	}
}

// TestAdoptBuildFailureIsFinishedByTheNextUpdate: an adoption that landed the source but
// failed its build leaves a work tree that has "nothing to pull" on the next click. The
// same marker has to make that click compile rather than report "already up to date".
func TestAdoptBuildFailureIsFinishedByTheNextUpdate(t *testing.T) {
	requireGit(t)
	tr, dir := newAdoptFixture(t)
	fake := scriptFakeGo(t)
	fake.failBuild(t)

	o := Options{Root: dir, Repo: tr.upstream, BinaryName: Binary}
	res, err := Adopt(context.Background(), o, nil)
	ue, ok := err.(*Error)
	if !ok || ue.Reason != "build_failed" {
		t.Fatalf("adopt: err=%v result=%+v, want build_failed", err, res)
	}
	snap, err := Status(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.BuildPending {
		t.Error("the adopted tree owes a build; Status must say so")
	}

	fake.succeed(t)
	res, err = Apply(context.Background(), o, nil)
	if err != nil {
		t.Fatalf("retry failed: %v (result %+v)", err, res)
	}
	if !res.BinaryBuilt {
		t.Fatalf("retry result = %+v, want the owed build", res)
	}
	if got := readFile(t, filepath.Join(dir, Binary)); !strings.Contains(got, "fake binary") {
		t.Errorf("binary after retry = %q, want the compiled one", got)
	}
	if buildPending(dir) {
		t.Error("a finished build must clear the pending marker")
	}
}

// TestRetryWithoutToolchainKeepsSayingSoAndThenBuilds: the no_toolchain message tells the
// operator "安装 go 后再点一次更新即可补上二进制". Until the fix that retry answered
// success without compiling; the promise is only real if the click after installing go
// actually builds - and the clicks before it keep an honest refusal.
func TestRetryWithoutToolchainKeepsSayingSoAndThenBuilds(t *testing.T) {
	requireGit(t)
	tr := newTree(t)
	hideToolchain(t)

	bin := filepath.Join(tr.install, Binary)
	writeFile(t, bin, "old binary bytes")
	tr.upstreamCommit(t, "upstream moves", map[string]string{"internal_service.go": "package service\n\nconst Version = \"2\"\n"})

	if _, err := Apply(context.Background(), buildOpts(tr), nil); err == nil {
		t.Fatal("a tree without a toolchain must refuse the build, not report success")
	} else if ue, ok := err.(*Error); !ok || ue.Reason != "no_toolchain" {
		t.Fatalf("want no_toolchain, got %v", err)
	}
	// Still no go: the retry keeps refusing.
	if _, err := Apply(context.Background(), buildOpts(tr), nil); err == nil {
		t.Fatal("a retry without a toolchain must keep refusing, not report success")
	}

	// Toolchain installed: the same click now does what the message said it would.
	scriptFakeGo(t)
	res, err := Apply(context.Background(), buildOpts(tr), nil)
	if err != nil {
		t.Fatalf("retry with a toolchain failed: %v (result %+v)", err, res)
	}
	if !res.BinaryBuilt {
		t.Errorf("result = %+v, want the binary built on retry", res)
	}
	if got := readFile(t, bin); !strings.Contains(got, "fake binary") {
		t.Errorf("binary = %q, want the compiled one", got)
	}
}
