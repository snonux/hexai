package askcli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snonux/hexai/internal/filelock"
)

type lockResult struct {
	unlock func() error
	err    error
}

func TestAcquireAskRepoLock_SerializesConcurrentHolders(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	var maxHeld int32
	var cur int32
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock, err := acquireAskRepoLock(context.Background(), tmp)
			if err != nil {
				t.Errorf("lock: %v", err)
				return
			}
			defer func() { _ = unlock() }()
			n := atomic.AddInt32(&cur, 1)
			for {
				old := atomic.LoadInt32(&maxHeld)
				if n <= old || atomic.CompareAndSwapInt32(&maxHeld, old, n) {
					break
				}
			}
			time.Sleep(25 * time.Millisecond)
			atomic.AddInt32(&cur, -1)
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxHeld); got != 1 {
		t.Fatalf("max concurrent lock holders = %d, want 1", got)
	}
}

func TestAcquireAskRepoLock_StaleMetadataDoesNotRotateContendedLockFile(t *testing.T) {
	tmp := t.TempDir()
	holder, lockPath, origInfo := prepareContendedStaleLock(t, tmp)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resultCh := acquireLockAsync(ctx, tmp)

	select {
	case result := <-resultCh:
		if result.unlock != nil {
			_ = result.unlock()
		}
		t.Fatalf("lock acquired while holder still held lock: %v", result.err)
	case <-time.After(40 * time.Millisecond):
	}

	curInfo, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat contended lock: %v", err)
	}
	if !os.SameFile(origInfo, curInfo) {
		t.Fatal("contended lock file was replaced while locked")
	}

	releaseContendedLock(t, holder)

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("contender lock: %v", result.err)
	}
	if result.unlock == nil {
		t.Fatal("contender returned nil unlock")
	}
	if err := result.unlock(); err != nil {
		t.Fatalf("contender unlock: %v", err)
	}
}

// TestAcquireAskRepoLock_ContextCancelledWhileBlocked verifies the retry loop
// honors context cancellation while it is parked on the per-iteration timer.
// This guards the time.After-based wait that replaced the unsafe timer reuse:
// cancellation must win the select and return ctx.Err() promptly.
func TestAcquireAskRepoLock_ContextCancelledWhileBlocked(t *testing.T) {
	tmp := t.TempDir()
	holder, _, _ := prepareContendedStaleLock(t, tmp)
	defer releaseContendedLock(t, holder)

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := acquireLockAsync(ctx, tmp)

	// Let the contender enter the retry loop, then cancel while it waits.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case result := <-resultCh:
		if result.unlock != nil {
			_ = result.unlock()
			t.Fatal("lock acquired despite cancellation")
		}
		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", result.err)
		}
	case <-time.After(time.Second):
		t.Fatal("acquireAskRepoLock did not return after cancellation")
	}
}

// TestAcquireAskRepoLock_GitfileWorktree places the lock in the common git dir
// named via a worktree gitfile + commondir (agent-isolated / linked worktree),
// not under the .git file path and not in the worktree-private metadata dir.
func TestAcquireAskRepoLock_GitfileWorktree(t *testing.T) {
	tmp := t.TempDir()
	_, worktree, commonGit, wtPrivate := linkedWorktreeLayout(t, tmp)

	unlock, err := acquireAskRepoLock(context.Background(), worktree)
	if err != nil {
		t.Fatalf("lock with gitfile: %v", err)
	}
	defer func() { _ = unlock() }()

	wantLock := filepath.Join(commonGit, askRepoLockFile)
	if _, err := os.Stat(wantLock); err != nil {
		t.Fatalf("lock file not created in common git dir: %v", err)
	}
	privateLock := filepath.Join(wtPrivate, askRepoLockFile)
	if _, err := os.Stat(privateLock); err == nil {
		t.Fatal("lock was placed in worktree-private git dir; want common dir")
	}
	info, err := os.Stat(filepath.Join(worktree, ".git"))
	if err != nil {
		t.Fatalf("stat .git after lock: %v", err)
	}
	if info.IsDir() {
		t.Fatal(".git was replaced with a directory; gitfile layout destroyed")
	}
}

func TestAcquireAskRepoLock_GitfileRelativePath(t *testing.T) {
	tmp := t.TempDir()
	worktree := filepath.Join(tmp, "wt")
	commonGit := filepath.Join(tmp, "common.git")
	wtPrivate := filepath.Join(commonGit, "worktrees", "wt")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wtPrivate, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPrivate, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Relative gitdir from worktree to private metadata dir.
	rel, err := filepath.Rel(worktree, wtPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireAskRepoLock(context.Background(), worktree)
	if err != nil {
		t.Fatalf("lock with relative gitfile: %v", err)
	}
	_ = unlock()

	if _, err := os.Stat(filepath.Join(commonGit, askRepoLockFile)); err != nil {
		t.Fatalf("expected lock in common git dir: %v", err)
	}
}

// TestAcquireAskRepoLock_MainAndWorktreeShareLock verifies the repo lock is the
// same file for the main checkout and a linked worktree, so they serialize.
func TestAcquireAskRepoLock_MainAndWorktreeShareLock(t *testing.T) {
	tmp := t.TempDir()
	mainRoot, worktree, commonGit, _ := linkedWorktreeLayout(t, tmp)

	unlockMain, err := acquireAskRepoLock(context.Background(), mainRoot)
	if err != nil {
		t.Fatalf("main lock: %v", err)
	}
	mainInfo, err := os.Stat(filepath.Join(commonGit, askRepoLockFile))
	if err != nil {
		t.Fatalf("stat main lock: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resultCh := acquireLockAsync(ctx, worktree)

	select {
	case result := <-resultCh:
		if result.unlock != nil {
			_ = result.unlock()
		}
		t.Fatalf("worktree acquired lock while main still held it: %v", result.err)
	case <-time.After(40 * time.Millisecond):
	}

	wtInfo, err := os.Stat(filepath.Join(commonGit, askRepoLockFile))
	if err != nil {
		t.Fatalf("stat shared lock: %v", err)
	}
	if !os.SameFile(mainInfo, wtInfo) {
		t.Fatal("main and worktree did not share the same lock file")
	}

	if err := unlockMain(); err != nil {
		t.Fatalf("main unlock: %v", err)
	}
	result := <-resultCh
	if result.err != nil {
		t.Fatalf("worktree lock after main release: %v", result.err)
	}
	if result.unlock == nil {
		t.Fatal("worktree returned nil unlock")
	}
	_ = result.unlock()
}

func TestResolveGitDir_RejectsInvalidGitfile(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, ".git"), []byte("not-a-gitfile\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveGitDir(tmp)
	if err == nil {
		t.Fatal("expected error for invalid gitfile")
	}
}

func TestResolveGitDir_RejectsMissingGitdirTarget(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist")
	body := "gitdir: " + missing + "\n"
	if err := os.WriteFile(filepath.Join(tmp, ".git"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveGitDir(tmp)
	if err == nil {
		t.Fatal("expected error when gitdir target is missing")
	}
}

func TestResolveGitDir_RejectsGitdirThatIsAFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "not-a-dir")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := "gitdir: " + target + "\n"
	if err := os.WriteFile(filepath.Join(tmp, ".git"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveGitDir(tmp)
	if err == nil {
		t.Fatal("expected error when gitdir points at a file")
	}
}

func TestParseGitfile_EmptyPath(t *testing.T) {
	_, err := parseGitfile("/tmp", []byte("gitdir:   \n"))
	if err == nil {
		t.Fatal("expected error for empty gitdir path")
	}
}

func TestParseGitfile_UsesFirstLineOnly(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "gitdir: " + real + "\njunk-should-be-ignored\n"
	got, err := parseGitfile(tmp, []byte(body))
	if err != nil {
		t.Fatalf("parseGitfile: %v", err)
	}
	if got != real {
		t.Fatalf("got %q, want %q", got, real)
	}
}

func TestParseGitfile_RejectsWrongCasePrefix(t *testing.T) {
	_, err := parseGitfile("/tmp", []byte("GITDIR: /tmp\n"))
	if err == nil {
		t.Fatal("expected error for case-mismatched gitdir prefix")
	}
}

func TestParseGitfile_RejectsMissingSpaceAfterColon(t *testing.T) {
	_, err := parseGitfile("/tmp", []byte("gitdir:/tmp\n"))
	if err == nil {
		t.Fatal("expected error when space after gitdir: is missing")
	}
}

func TestResolveGitCommonDir_EmptyCommondir(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "commondir"), []byte("  \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveGitCommonDir(tmp)
	if err == nil {
		t.Fatal("expected error for empty commondir")
	}
}

func TestResolveGitCommonDir_MissingTarget(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "nope")
	if err := os.WriteFile(filepath.Join(tmp, "commondir"), []byte(missing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveGitCommonDir(tmp)
	if err == nil {
		t.Fatal("expected error for missing commondir target")
	}
}

func TestResolveGitCommonDir_TargetIsFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "file")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "commondir"), []byte(target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveGitCommonDir(tmp)
	if err == nil {
		t.Fatal("expected error when commondir points at a file")
	}
}

func TestResolveGitCommonDir_AbsolutePath(t *testing.T) {
	tmp := t.TempDir()
	common := filepath.Join(tmp, "common")
	private := filepath.Join(tmp, "private")
	if err := os.MkdirAll(common, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(private, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(private, "commondir"), []byte(common+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveGitCommonDir(private)
	if err != nil {
		t.Fatalf("resolveGitCommonDir: %v", err)
	}
	if got != common {
		t.Fatalf("got %q, want %q", got, common)
	}
}

func TestResolveAskLockDirViaGit_RealRepo(t *testing.T) {
	tmp := t.TempDir()
	runGit(t, tmp, "init")
	got, err := resolveAskLockDirViaGit(context.Background(), tmp)
	if err != nil {
		t.Fatalf("via git: %v", err)
	}
	want := filepath.Join(tmp, ".git")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolveAskLockDirViaGit_IgnoresGITDIREnv(t *testing.T) {
	tmp := t.TempDir()
	repoA := filepath.Join(tmp, "a")
	repoB := filepath.Join(tmp, "b")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoA, "init")
	runGit(t, repoB, "init")

	t.Setenv("GIT_DIR", filepath.Join(repoB, ".git"))
	got, err := resolveAskLockDirViaGit(context.Background(), repoA)
	if err != nil {
		t.Fatalf("via git: %v", err)
	}
	want := filepath.Join(repoA, ".git")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("GIT_DIR leaked: got %q, want %q", got, want)
	}
}

func TestResolveAskLockDir_CancelledContextDoesNotFallback(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolveAskLockDir(ctx, tmp)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}

	unlock, err := acquireAskRepoLock(ctx, tmp)
	if unlock != nil {
		_ = unlock()
		t.Fatal("acquired lock with cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("acquire err = %v, want context.Canceled", err)
	}
}

func TestResolveAskLockDir_ExpiredDeadlineDoesNotFallback(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := resolveAskLockDir(ctx, tmp)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestScrubGitOverrideEnv(t *testing.T) {
	in := []string{
		"PATH=/bin",
		"GIT_DIR=/other/.git",
		"Git_Dir=/mixed/.git",
		"GIT_WORK_TREE=/other",
		"HOME=/home/test",
		"GIT_COMMON_DIR=/x",
	}
	got := scrubGitOverrideEnv(in)
	want := []string{"PATH=/bin", "HOME=/home/test"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAcquireAskRepoLock_RealGitWorktreeSharesLock(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	tmp := t.TempDir()
	mainRoot := filepath.Join(tmp, "main")
	if err := os.MkdirAll(mainRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, mainRoot, "init")
	runGit(t, mainRoot, "commit", "--allow-empty", "-m", "init")
	worktree := filepath.Join(tmp, "wt")
	runGit(t, mainRoot, "worktree", "add", "--detach", worktree, "HEAD")

	unlockMain, err := acquireAskRepoLock(context.Background(), mainRoot)
	if err != nil {
		t.Fatalf("main lock: %v", err)
	}
	commonLock := filepath.Join(mainRoot, ".git", askRepoLockFile)
	mainInfo, err := os.Stat(commonLock)
	if err != nil {
		t.Fatalf("stat common lock: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resultCh := acquireLockAsync(ctx, worktree)
	select {
	case result := <-resultCh:
		if result.unlock != nil {
			_ = result.unlock()
		}
		t.Fatalf("worktree acquired lock while main held it: %v", result.err)
	case <-time.After(40 * time.Millisecond):
	}
	wtInfo, err := os.Stat(commonLock)
	if err != nil {
		t.Fatalf("stat lock from worktree side: %v", err)
	}
	if !os.SameFile(mainInfo, wtInfo) {
		t.Fatal("real git worktree did not share main lock file")
	}
	_ = unlockMain()
	result := <-resultCh
	if result.err != nil {
		t.Fatalf("worktree lock: %v", result.err)
	}
	_ = result.unlock()
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// linkedWorktreeLayout builds a main checkout + linked worktree that share one
// common .git directory, mirroring git worktree / agent isolation layout.
func linkedWorktreeLayout(t *testing.T, tmp string) (mainRoot, worktree, commonGit, wtPrivate string) {
	t.Helper()
	mainRoot = filepath.Join(tmp, "main")
	commonGit = filepath.Join(mainRoot, ".git")
	wtPrivate = filepath.Join(commonGit, "worktrees", "agent")
	worktree = filepath.Join(tmp, "agent-wt")
	for _, dir := range []string{mainRoot, wtPrivate, worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(wtPrivate, "commondir"), []byte("../..\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitfile := "gitdir: " + wtPrivate + "\n"
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte(gitfile), 0o644); err != nil {
		t.Fatal(err)
	}
	return mainRoot, worktree, commonGit, wtPrivate
}

func prepareContendedStaleLock(t *testing.T, gitRoot string) (*os.File, string, os.FileInfo) {
	t.Helper()
	lockDir := filepath.Join(gitRoot, ".git")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(lockDir, askRepoLockFile)
	holder, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := filelock.TryExclusive(holder); err != nil {
		t.Fatalf("holder lock: %v", err)
	}
	if err := writeLockMetadata(holder, 999999, "ask"); err != nil {
		t.Fatalf("write stale metadata: %v", err)
	}
	origInfo, err := os.Stat(lockPath)
	if err != nil {
		t.Fatalf("stat original lock: %v", err)
	}
	return holder, lockPath, origInfo
}

func acquireLockAsync(ctx context.Context, gitRoot string) <-chan lockResult {
	resultCh := make(chan lockResult, 1)
	go func() {
		unlock, err := acquireAskRepoLock(ctx, gitRoot)
		resultCh <- lockResult{unlock: unlock, err: err}
	}()
	return resultCh
}

func releaseContendedLock(t *testing.T, holder *os.File) {
	t.Helper()
	if err := filelock.UnlockExclusive(holder); err != nil {
		t.Fatalf("release holder lock: %v", err)
	}
	if err := holder.Close(); err != nil {
		t.Fatalf("close holder file: %v", err)
	}
}
