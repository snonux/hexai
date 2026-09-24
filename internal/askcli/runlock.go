package askcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/snonux/hexai/internal/filelock"
)

const askRepoLockFile = "hexai-ask.lock"

func lockProcessLabel() string {
	if exe, err := os.Executable(); err == nil {
		if b := filepath.Base(exe); b != "" && b != "." {
			return b
		}
	}
	if b := filepath.Base(os.Args[0]); b != "" {
		return b
	}
	return "ask"
}

func readLockHolderPID(f *os.File) int {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0
	}
	var buf [64]byte
	n, err := f.Read(buf[:])
	if err != nil && !errors.Is(err, io.EOF) {
		return 0
	}
	line := strings.TrimSpace(string(buf[:n]))
	if line == "" {
		return 0
	}
	end := strings.IndexAny(line, "\n\r \t")
	if end >= 0 {
		line = line[:end]
	}
	pid, err := strconv.Atoi(line)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

func writeLockMetadata(f *os.File, pid int, comm string) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err := fmt.Fprintf(f, "%d\n%s\n", pid, comm)
	if err != nil {
		return err
	}
	return f.Sync()
}

// askLockRetryInterval is the backoff between successive non-blocking lock attempts.
const askLockRetryInterval = 5 * time.Millisecond

// waitOrAcquireAskLockFD tries to take an exclusive lock on f, or blocks until ctx ends.
// On success it writes lock metadata and returns an unlock function (which closes f).
func waitOrAcquireAskLockFD(
	ctx context.Context,
	f *os.File,
	comm string,
) (func() error, error) {
	for {
		if err := ctx.Err(); err != nil {
			_ = f.Close()
			return nil, err
		}
		err := filelock.TryExclusive(f)
		if err == nil {
			return finishAskLockAcquire(f, comm)
		}
		if !errors.Is(err, filelock.ErrWouldBlock) {
			_ = f.Close()
			return nil, fmt.Errorf("ask lock: %w", err)
		}
		noteStaleLockHolder(f, comm)
		if err := waitAskLockRetry(ctx, f); err != nil {
			return nil, err
		}
	}
}

func finishAskLockAcquire(f *os.File, comm string) (func() error, error) {
	if werr := writeLockMetadata(f, os.Getpid(), comm); werr != nil {
		_ = filelock.UnlockExclusive(f)
		_ = f.Close()
		return nil, fmt.Errorf("ask lock: write metadata: %w", werr)
	}
	return func() error {
		uErr := filelock.UnlockExclusive(f)
		cErr := f.Close()
		return errors.Join(uErr, cErr)
	}, nil
}

func noteStaleLockHolder(f *os.File, comm string) {
	pid := readLockHolderPID(f)
	// Keep waiting even if metadata appears stale: removing a contended lock file can
	// split ownership across different inodes and break serialization guarantees.
	if pid > 0 && lockHolderIsStale(pid, comm) {
		// Intentional no-op: contention is resolved only by waiting for flock release.
	}
}

// waitAskLockRetry parks until the retry interval elapses or ctx is cancelled.
// Uses time.After (not Reset) to avoid the documented Go timer hazard of a stale
// value already queued on a reused timer's channel.
func waitAskLockRetry(ctx context.Context, f *os.File) error {
	select {
	case <-ctx.Done():
		_ = f.Close()
		return ctx.Err()
	case <-time.After(askLockRetryInterval):
		return nil
	}
}

// acquireAskRepoLock serializes ask CLI access for a git repository. It places an
// advisory lock under the repository's common git directory (shared across linked
// worktrees and agent checkouts whose .git is a gitfile) and records holder PID
// plus process name for stale detection.
func acquireAskRepoLock(ctx context.Context, gitRoot string) (func() error, error) {
	lockDir, err := resolveAskLockDir(ctx, gitRoot)
	if err != nil {
		return nil, fmt.Errorf("ask lock: resolve lock dir: %w", err)
	}
	lockPath := filepath.Join(lockDir, askRepoLockFile)

	comm := lockProcessLabel()

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("ask lock: open %s: %w", lockPath, err)
	}
	return waitOrAcquireAskLockFD(ctx, f, comm)
}

// resolveAskLockDir returns the directory that should hold hexai-ask.lock for
// gitRoot. Prefer git's common dir so main checkouts and linked worktrees of
// the same repo serialize on one lock file. Fall back to filesystem parsing
// when git is unavailable (e.g. synthetic test layouts). Context cancellation
// or deadline is never papered over by the filesystem fallback.
func resolveAskLockDir(ctx context.Context, gitRoot string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	viaGit, gerr := resolveAskLockDirViaGit(ctx, gitRoot)
	if gerr == nil {
		return viaGit, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if errors.Is(gerr, context.Canceled) || errors.Is(gerr, context.DeadlineExceeded) {
		return "", gerr
	}
	gitDir, err := resolveGitDir(gitRoot)
	if err != nil {
		return "", fmt.Errorf("%w (git: %v)", err, gerr)
	}
	dir, err := resolveGitCommonDir(gitDir)
	if err != nil {
		return "", fmt.Errorf("%w (git: %v)", err, gerr)
	}
	return dir, nil
}

// resolveGitDir returns the per-worktree (or main) metadata directory for gitRoot.
// When .git is a directory it is returned; when .git is a gitfile (worktrees and
// some agent-isolated checkouts), the path after "gitdir:" is resolved.
func resolveGitDir(gitRoot string) (string, error) {
	gitPath := filepath.Join(gitRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return gitPath, nil
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is neither a directory nor a regular file", gitPath)
	}
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", err
	}
	return parseGitfile(gitRoot, data)
}

// resolveGitCommonDir follows a worktree "commondir" pointer when present so the
// lock lives in the shared repository git directory, not the worktree-private one.
func resolveGitCommonDir(gitDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		if os.IsNotExist(err) {
			return gitDir, nil
		}
		return "", err
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return "", fmt.Errorf("empty commondir in %s", gitDir)
	}
	dir := raw
	if !filepath.IsAbs(dir) {
		base, evalErr := filepath.EvalSymlinks(gitDir)
		if evalErr != nil {
			if pathHasDotDot(raw) {
				return "", fmt.Errorf("eval symlinks %s: %w", gitDir, evalErr)
			}
			base = gitDir
		}
		dir = filepath.Join(base, dir)
	}
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("commondir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("commondir %s is not a directory", dir)
	}
	return dir, nil
}

func resolveAskLockDirViaGit(ctx context.Context, gitRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", gitRoot, "rev-parse", "--git-common-dir")
	cmd.Env = scrubGitOverrideEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", err
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("empty git-common-dir")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(gitRoot, dir)
	}
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("git-common-dir %s is not a directory", dir)
	}
	return dir, nil
}

// scrubGitOverrideEnv drops variables that would make `git -C <root>` resolve a
// different repository than the given working tree (absolute GIT_DIR, etc.).
// Key matching is case-insensitive so Windows-style mixed-case names are scrubbed too.
func scrubGitOverrideEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, e := range environ {
		key, _, ok := strings.Cut(e, "=")
		if !ok {
			out = append(out, e)
			continue
		}
		switch {
		case strings.EqualFold(key, "GIT_DIR"),
			strings.EqualFold(key, "GIT_COMMON_DIR"),
			strings.EqualFold(key, "GIT_WORK_TREE"),
			strings.EqualFold(key, "GIT_OBJECT_DIRECTORY"),
			strings.EqualFold(key, "GIT_INDEX_FILE"),
			strings.EqualFold(key, "GIT_ALTERNATE_OBJECT_DIRECTORIES"),
			strings.EqualFold(key, "GIT_QUARANTINE_PATH"):
			continue
		}
		out = append(out, e)
	}
	return out
}

const gitfilePrefix = "gitdir: "

// parseGitfile parses a .git gitfile ("gitdir: <path>" on the first line) and
// returns the absolute metadata directory. Relative paths are resolved against
// gitRoot. Matching git, the prefix is case-sensitive and requires a space
// after the colon.
func parseGitfile(gitRoot string, data []byte) (string, error) {
	line := data
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		line = data[:i]
	}
	line = bytes.TrimRight(line, "\r")
	content := string(line)
	if !strings.HasPrefix(content, gitfilePrefix) {
		return "", fmt.Errorf("invalid gitfile: missing %q prefix", gitfilePrefix)
	}
	raw := strings.TrimSpace(content[len(gitfilePrefix):])
	if raw == "" {
		return "", fmt.Errorf("invalid gitfile: empty gitdir path")
	}
	dir := raw
	if !filepath.IsAbs(dir) {
		base, evalErr := filepath.EvalSymlinks(gitRoot)
		if evalErr != nil {
			if pathHasDotDot(raw) {
				return "", fmt.Errorf("eval symlinks %s: %w", gitRoot, evalErr)
			}
			base = gitRoot
		}
		dir = filepath.Join(base, dir)
	}
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("gitdir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("gitdir %s is not a directory", dir)
	}
	return dir, nil
}

func pathHasDotDot(p string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(p), "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}
