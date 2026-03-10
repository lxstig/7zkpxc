package sevenzip

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

// DefaultTimeout is the maximum time a 7z operation may run before being killed.
// Large archives may need a longer timeout; callers can use RunWithTimeout directly.
const DefaultTimeout = 4 * time.Hour

// Run executes a 7z command with secure password input via PTY.
// Uses DefaultTimeout. For custom timeouts use RunWithTimeout.
func Run(binaryPath string, password []byte, args []string) error {
	return RunWithTimeout(context.Background(), binaryPath, password, args, DefaultTimeout)
}

// RunWithTimeout executes a 7z command with a context deadline.
// The process is forcefully killed if the deadline is exceeded.
func RunWithTimeout(ctx context.Context, binaryPath string, password []byte, args []string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)

	// Force English locale to detect prompts reliably regardless of user locale
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	done := make(chan error, 1)

	// passwordSent is closed once the password has been written to the PTY.
	// The stdin bridge goroutine waits for this signal before forwarding user
	// input, ensuring the user cannot accidentally type before the password
	// prompt is handled.
	passwordSent := make(chan struct{})

	// stdin bridge: forwards user keystrokes to the PTY after the password has
	// been sent. This allows 7z's interactive prompts (e.g.
	// "(Y)es / (N)o / (A)lways / (S)kip all / (Q)uit?") to be answered.
	go func() {
		<-passwordSent
		_, _ = io.Copy(ptmx, os.Stdin)
	}()

	// Output processor goroutine: intercepts password prompts, suppresses echo
	go func() {
		defer close(done)

		buf := make([]byte, 32*1024) // 32 KB — large enough to avoid per-byte reads
		suppressUntilNewline := false
		sentPassword := false

		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				lowerChunk := bytes.ToLower(chunk)

				// Detect password prompt (handles "Enter password" and "Reenter password")
				if !suppressUntilNewline && (bytes.Contains(lowerChunk, []byte("enter password")) ||
					bytes.Contains(lowerChunk, []byte("password:"))) {

					_, _ = os.Stdout.Write(chunk)

					// Small delay to ensure the prompt is flushed before writing
					time.Sleep(10 * time.Millisecond)

					_, _ = ptmx.Write(password)
					_, _ = ptmx.Write([]byte("\n"))
					suppressUntilNewline = true

					// Unblock the stdin bridge on first password send only
					if !sentPassword {
						sentPassword = true
						close(passwordSent)
					}
					continue
				}

				if suppressUntilNewline {
					// Suppress echo until we see the newline that terminates it
					nlIdx := bytes.IndexAny(chunk, "\n\r")
					if nlIdx != -1 {
						suppressUntilNewline = false
						if nlIdx+1 < len(chunk) {
							_, _ = os.Stdout.Write(chunk[nlIdx+1:])
						}
					}
					continue
				}

				_, _ = os.Stdout.Write(chunk)
			}

			if err != nil {
				if err != io.EOF {
					_ = err // PTY read errors on process exit are expected
				}
				break
			}
		}

		// If the process exited before a password prompt was ever seen
		// (e.g. unencrypted archive), unblock the stdin bridge so it exits cleanly.
		if !sentPassword {
			close(passwordSent)
		}
	}()

	errWait := cmd.Wait()
	<-done

	// Distinguish timeout from other errors
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("7z operation timed out after %s", timeout)
	}

	return errWait
}

