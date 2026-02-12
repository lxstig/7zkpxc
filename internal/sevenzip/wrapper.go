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
		passwordSendCount := 0
		suppressNextLine := false

		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				chunk := buf[:n]
				lowerChunk := bytes.ToLower(chunk)

				// Detect password prompt (handles both "Enter password" and "Reenter password")
				if bytes.Contains(lowerChunk, []byte("enter password")) ||
					bytes.Contains(lowerChunk, []byte("password:")) {

					// Echo prompt to stdout (but not password)
					_, _ = os.Stdout.Write(chunk)

					// Small delay to ensure prompt is flushed
					time.Sleep(10 * time.Millisecond)

					// Write password + newline
					_, _ = ptmx.Write([]byte(password + "\n"))
					passwordSendCount++
					suppressNextLine = true // Don't echo the password line
					continue
				}

				// Suppress echo of password (if terminal echoes it back)
				if suppressNextLine {
					// Check if this chunk contains the password (could be echoed)
					if bytes.Contains(chunk, []byte(password)) {
						suppressNextLine = false
						continue
					}
					// If chunk is just whitespace/newline, skip
					if bytes.Equal(bytes.TrimSpace(chunk), []byte{}) {
						suppressNextLine = false
						continue
					}
					suppressNextLine = false
				}

				// Echo to stdout for user visibility
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

		_ = passwordSendCount // available for future diagnostics
	}()

	// We block here until process exit
	errWait := cmd.Wait()

	// Wait for goroutine to finish
	<-done

	return errWait
}
