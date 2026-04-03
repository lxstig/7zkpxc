package sevenzip

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
)

// DefaultTimeout is the maximum time a 7z operation may run before being killed.
// Large archives may need a longer timeout; callers can use RunWithTimeout directly.
const DefaultTimeout = 4 * time.Hour

// Run executes a 7z command with secure password input via PTY.
// Uses DefaultTimeout. For custom timeouts use RunWithTimeout.
func Run(binaryPath string, password []byte, args []string) error {
	_, err := runWithTimeoutInternal(context.Background(), binaryPath, password, args, DefaultTimeout, false)
	return err
}

// RunWithTimeout executes a 7z command with a context deadline.
// The process is forcefully killed if the deadline is exceeded.
func RunWithTimeout(ctx context.Context, binaryPath string, password []byte, args []string, timeout time.Duration) error {
	_, err := runWithTimeoutInternal(ctx, binaryPath, password, args, timeout, false)
	return err
}

// PasswordMatch indicates the result of verifying an archive password.
type PasswordMatch int

const (
	MatchCorrect     PasswordMatch = iota // Password was correct and data decrypted.
	MatchUnencrypted                      // Archive did not require a password.
	MatchFailed                           // Wrong password or other runtime error.
)

// VerifyPassword performs a silent test using 7-zip's list command to check header decryption.
func VerifyPassword(binaryPath string, password []byte, archivePath string) (PasswordMatch, error) {
	args := []string{"l", "-slt", "-ba", archivePath}
	prompted, err := runWithTimeoutInternal(context.Background(), binaryPath, password, args, DefaultTimeout, true)
	if err == nil {
		if prompted {
			return MatchCorrect, nil
		}
		return MatchUnencrypted, nil
	}
	return MatchFailed, err
}

// runWithTimeoutInternal returns (passwordWasPrompted, error).
// passwordWasPrompted is true when 7z actually asked for a password,
// false when the archive is unencrypted and 7z never prompted.
func runWithTimeoutInternal(ctx context.Context, binaryPath string, password []byte, args []string, timeout time.Duration, silent bool) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)

	// Force English locale to detect prompts reliably regardless of user locale
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return false, err
	}
	defer func() { _ = ptmx.Close() }()

	done := make(chan error, 1)

	// passwordSent is closed once the password has been written to the PTY.
	// The stdin bridge goroutine waits for this signal before forwarding user
	// input, ensuring the user cannot accidentally type before the password
	// prompt is handled.
	passwordSent := make(chan struct{})

	// Track whether 7z actually prompted for a password (atomic for goroutine safety)
	var prompted atomic.Bool

	go bridgeStdin(ctx, ptmx, passwordSent)
	go processOutput(ptmx, password, passwordSent, silent, done, &prompted)

	errWait := cmd.Wait()
	<-done

	wasPrompted := prompted.Load()

	// Distinguish timeout from other errors
	if ctx.Err() == context.DeadlineExceeded {
		return wasPrompted, fmt.Errorf("7z operation timed out after %s", timeout)
	}

	if errWait != nil {
		var exitErr *exec.ExitError
		if errors.As(errWait, &exitErr) {
			code := exitErr.ExitCode()
			return wasPrompted, fmt.Errorf("7z exited with code %d (%s)", code, sevenZipExitCodeDesc(code))
		}
	}

	return wasPrompted, errWait
}

// sevenZipExitCodeDesc returns a human-readable description for 7z exit codes.
func sevenZipExitCodeDesc(code int) string {
	switch code {
	case 0:
		return "No error"
	case 1:
		return "Warning (Non fatal error(s)). For example, one or more files were locked by some other application, so they were not compressed."
	case 2:
		return "Fatal error"
	case 7:
		return "Command line error"
	case 8:
		return "Not enough memory for operation"
	case 255:
		return "User stopped the process"
	default:
		return "Unknown error"
	}
}

// bridgeStdin forwards user keystrokes to the PTY after the password has
// been sent. This allows 7z's interactive prompts (e.g.
// "(Y)es / (N)o / (A)lways / (S)kip all / (Q)uit?") to be answered.
func bridgeStdin(ctx context.Context, ptmx *os.File, passwordSent <-chan struct{}) {
	select {
	case <-passwordSent:
	case <-ctx.Done():
		return
	}

	// A crude but effective way to prevent the goroutine from leaking indefinitely
	// when os.Stdin blocks. We read 1 byte at a time in a separate sub-routine
	// and check context cancellation.
	ch := make(chan []byte)
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				b := make([]byte, n)
				copy(b, buf[:n])
				ch <- b
			}
			if err != nil {
				close(ch)
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case b, ok := <-ch:
			if !ok {
				return
			}
			_, _ = ptmx.Write(b)
		}
	}
}

// processOutput intercepts password prompts and suppresses token echo.
func processOutput(ptmx *os.File, password []byte, passwordSent chan<- struct{}, silent bool, done chan<- error, prompted *atomic.Bool) {
	defer close(done)

	buf := make([]byte, 32*1024) // 32 KB — large enough to avoid per-byte reads
	suppressUntilNewline := false
	sentPassword := false

	defer func() {
		// If the process exited before a password prompt was ever seen
		// (e.g. unencrypted archive), unblock the stdin bridge so it exits cleanly.
		if !sentPassword {
			close(passwordSent)
		}
	}()

	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			lowerChunk := bytes.ToLower(chunk)

			// Detect password prompt (handles "Enter password" and "Reenter password")
			if !suppressUntilNewline && (bytes.Contains(lowerChunk, []byte("enter password")) ||
				bytes.Contains(lowerChunk, []byte("password:"))) {

				if !silent {
					_, _ = os.Stdout.Write(chunk)
				}

				// Introduce a tiny delay so the OS PTY layer has time to apply tcsetattr (echo off).
				// Heavily-loaded CI runners may drop or garble prompt bytes if written instantly.
				time.Sleep(50 * time.Millisecond)

				_, _ = ptmx.Write(password)
				_, _ = ptmx.Write([]byte("\n"))
				suppressUntilNewline = true

				// Unblock the stdin bridge on first password send only
				if !sentPassword {
					sentPassword = true
					prompted.Store(true)
					close(passwordSent)
				}
				continue
			}

			if suppressUntilNewline {
				// Suppress echo until we see the newline that terminates it
				nlIdx := bytes.IndexAny(chunk, "\n\r")
				if nlIdx != -1 {
					suppressUntilNewline = false
					if nlIdx+1 < len(chunk) && !silent {
						_, _ = os.Stdout.Write(chunk[nlIdx+1:])
					}
				}
				continue
			}

			if !silent {
				_, _ = os.Stdout.Write(chunk)
			}
		}

		if err != nil {
			// PTY read errors on process exit are expected; break cleanly.
			break
		}
	}
}
