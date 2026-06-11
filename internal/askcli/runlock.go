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

	"codeberg.org/snonux/hexai/internal/filelock"
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
// advisory lock under .git and records holder PID plus process name for stale detection.
func acquireAskRepoLock(ctx context.Context, gitRoot string) (func() error, error) {
	lockPath := filepath.Join(gitRoot, ".git", askRepoLockFile)
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, fmt.Errorf("ask lock: mkdir: %w", err)
	}

	comm := lockProcessLabel()

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("ask lock: open %s: %w", lockPath, err)
	}
	return waitOrAcquireAskLockFD(ctx, f, comm)
}
