package sevenzip

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

// Run executes a 7z command with secure password input via PTY.
// This function is used for all 7z operations (a, x, l) that require password.
// It handles both single password prompts (extract/list) and double prompts (create/add).
func Run(password string, args []string) error {
	cmd := exec.Command("7z", args...)

	// Force English locale to detect prompt reliably
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	done := make(chan error, 1)

	// Output processor
	go func() {
		defer close(done)

		buf := make([]byte, 1024)
		suppressUntilNewline := false

		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				lowerChunk := bytes.ToLower(chunk)

				// Detect password prompt (handles both "Enter password" and "Reenter password")
				if !suppressUntilNewline && (bytes.Contains(lowerChunk, []byte("enter password")) ||
					bytes.Contains(lowerChunk, []byte("password:"))) {

					// Echo prompt to stdout
					_, _ = os.Stdout.Write(chunk)

					// Small delay to ensure prompt is flushed
					time.Sleep(10 * time.Millisecond)

					// Write password + newline
					_, _ = ptmx.Write([]byte(password + "\n"))
					suppressUntilNewline = true // Start suppressing echo
					continue
				}

				if suppressUntilNewline {
					// Search for newline which marks end of password echo
					nlIdx := bytes.IndexAny(chunk, "\n\r")
					if nlIdx != -1 {
						// Found newline!
						suppressUntilNewline = false

						// If there is content AFTER the newline, print it
						if nlIdx+1 < len(chunk) {
							_, _ = os.Stdout.Write(chunk[nlIdx+1:])
						}
					}
					// If no newline, we suppress the entire chunk (it's part of the password echo)
					continue
				}

				// Normal output
				_, _ = os.Stdout.Write(chunk)
			}

			if err != nil {
				if err != io.EOF {
					// PTY read error (process exited?)
					_ = err
				}
				break
			}
		}
	}()

	// We block here until process exit
	errWait := cmd.Wait()

	// Wait for goroutine to finish
	<-done

	return errWait
}
