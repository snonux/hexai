package askcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
		err := filelock.TryExclusive(f)
		if err == nil {
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
		if !errors.Is(err, filelock.ErrWouldBlock) {
			_ = f.Close()
			return nil, fmt.Errorf("ask lock: %w", err)
		}

		pid := readLockHolderPID(f)
		// Keep waiting even if metadata appears stale: removing a contended lock file can
		// split ownership across different inodes and break serialization guarantees.
		if pid > 0 && lockHolderIsStale(pid, comm) {
			// Intentional no-op: contention is resolved only by waiting for flock release.
		}

		// Use a fresh timer per iteration via time.After instead of reusing and
		// Reset()-ing a single timer. Reset() on a timer that may still be pending is
		// the documented Go timer hazard: a stale value can already be queued on the
		// channel and trigger a spurious early wake-up. A new timer each loop guarantees
		// a clean, full askLockRetryInterval delay (or ctx cancellation).
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(askLockRetryInterval):
		}
	}
}

// acquireAskRepoLock serializes ask CLI access for a git working copy. It uses an
// advisory lock under the real git metadata directory (the .git directory, or the
// directory named by a .git gitfile in worktrees / agent checkouts) and records
// holder PID plus process name for stale detection.
func acquireAskRepoLock(ctx context.Context, gitRoot string) (func() error, error) {
	gitDir, err := resolveGitDir(gitRoot)
	if err != nil {
		return nil, fmt.Errorf("ask lock: resolve git dir: %w", err)
	}
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		return nil, fmt.Errorf("ask lock: mkdir: %w", err)
	}
	lockPath := filepath.Join(gitDir, askRepoLockFile)

	comm := lockProcessLabel()

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("ask lock: open %s: %w", lockPath, err)
	}
	return waitOrAcquireAskLockFD(ctx, f, comm)
}

// resolveGitDir returns the repository metadata directory for gitRoot.
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

const gitfilePrefix = "gitdir:"

// parseGitfile parses a .git gitfile body ("gitdir: <path>") and returns the
// absolute metadata directory. Relative paths are resolved against gitRoot.
func parseGitfile(gitRoot string, data []byte) (string, error) {
	content := strings.TrimSpace(string(data))
	if len(content) < len(gitfilePrefix) ||
		!strings.EqualFold(content[:len(gitfilePrefix)], gitfilePrefix) {
		return "", fmt.Errorf("invalid gitfile: missing %q prefix", gitfilePrefix)
	}
	raw := strings.TrimSpace(content[len(gitfilePrefix):])
	if raw == "" {
		return "", fmt.Errorf("invalid gitfile: empty gitdir path")
	}
	dir := raw
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(gitRoot, dir)
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
