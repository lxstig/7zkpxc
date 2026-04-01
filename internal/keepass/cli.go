package keepass

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type Client struct {
	DatabasePath   string
	masterPassword []byte // zeroed on Close()
	passwordSet    bool
}

func New(dbPath string) *Client {
	return &Client{
		DatabasePath: dbPath,
	}
}

// buildCmd creates an exec.Cmd for keepassxc-cli enforcing English output
// so that error string matching (like "already exists") works consistently regardless of user locale.
func buildCmd(args ...string) *exec.Cmd {
	cmd := exec.Command("keepassxc-cli", args...)
	// Force English locale for parsing CLI output. Qt/KDE apps might need multiple variables.
	cmd.Env = append(os.Environ(),
		"LC_ALL=C",
		"LANGUAGE=en_US.UTF-8",
		"LANG=en_US.UTF-8",
	)
	return cmd
}

// Close securely wipes the master password from memory
func (c *Client) Close() {
	for i := range c.masterPassword {
		c.masterPassword[i] = 0
	}
	c.masterPassword = nil
	c.passwordSet = false
}

// getMasterPassword returns the master password.
// Be very careful when using this to avoid creating string copies that linger in memory.
func (c *Client) getMasterPassword() []byte {
	return c.masterPassword
}

func (c *Client) runCmd(args ...string) ([]byte, error) {
	for {
		if err := c.EnsureUnlocked(); err != nil {
			return nil, err
		}

		cmd := buildCmd(args...)
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}

		if err := cmd.Start(); err != nil {
			return nil, err
		}

		_, _ = stdin.Write(c.getMasterPassword())
		_, _ = stdin.Write([]byte("\n"))
		_ = stdin.Close()

		err = cmd.Wait()
		if err != nil {
			errStr := outBuf.String()
			// Check if the error is an incorrect master password
			if strings.Contains(errStr, "Invalid credentials") || strings.Contains(errStr, "HMAC mismatch") {
				fmt.Println("\033[31mError: Invalid KeePassXC master password. Please try again.\033[0m")
				c.clearMasterPassword()
				continue
			}
			return outBuf.Bytes(), err
		}
		return outBuf.Bytes(), nil
	}
}

func (c *Client) runCmdQuiet(args ...string) ([]byte, error) {
	for {
		if err := c.EnsureUnlocked(); err != nil {
			return nil, err
		}

		cmd := buildCmd(args...)
		var outBuf bytes.Buffer
		var errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, err
		}

		if err := cmd.Start(); err != nil {
			return nil, err
		}

		_, _ = stdin.Write(c.getMasterPassword())
		_, _ = stdin.Write([]byte("\n"))
		_ = stdin.Close()

		err = cmd.Wait()
		if err != nil {
			// keepassxc-cli prints the password prompt to stderr.
			var actualErrLines []string
			for _, line := range strings.Split(errBuf.String(), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if strings.HasSuffix(line, ":") && strings.Contains(line, filepath.Base(c.DatabasePath)) {
					continue
				}
				if line == "No results for that search term." || line == "Entry not found." {
					continue
				}
				actualErrLines = append(actualErrLines, line)
			}

			actualErrStr := strings.Join(actualErrLines, "\n")

			// Check if the error is an incorrect master password
			if strings.Contains(actualErrStr, "Invalid credentials") || strings.Contains(actualErrStr, "HMAC mismatch") {
				fmt.Println("\033[31mError: Invalid KeePassXC master password. Please try again.\033[0m")
				c.clearMasterPassword()
				continue
			}

			if actualErrStr != "" {
				return outBuf.Bytes(), fmt.Errorf("%w: %s", err, actualErrStr)
			}
			return outBuf.Bytes(), err
		}
		return outBuf.Bytes(), nil
	}
}

// clearMasterPassword zeroes and drops the cached master password
func (c *Client) clearMasterPassword() {
	for i := range c.masterPassword {
		c.masterPassword[i] = 0
	}
	c.masterPassword = nil
	c.passwordSet = false
}

// GeneratePassword creates a secure random password using keepassxc-cli generate.
// This delegates all cryptographic work to KeePassXC's audited generator.
// Flags: -L length, -l lowercase, -U uppercase, -n numbers, -s special characters.
func (c *Client) GeneratePassword(length int) ([]byte, error) {
	cmd := buildCmd("generate",
		"-L", strconv.Itoa(length),
		"-l", "-U", "-n", "-s",
	)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("keepassxc-cli generate failed: %w: %s", err, errBuf.String())
	}

	password := bytes.TrimSpace(outBuf.Bytes())
	if len(password) == 0 {
		return nil, fmt.Errorf("keepassxc-cli generate returned empty password")
	}

	// Create a copy to own the memory cleanly, so we can zero it later
	passCopy := make([]byte, len(password))
	copy(passCopy, password)

	// Best effort to zero out the buffer from `outBuf`
	outBytes := outBuf.Bytes()
	for i := range outBytes {
		outBytes[i] = 0
	}

	return passCopy, nil
}

// EnsureUnlocked prompts for master password if not set
func (c *Client) EnsureUnlocked() error {
	if c.passwordSet {
		return nil
	}

	dir := filepath.Dir(c.DatabasePath) + "/"
	base := filepath.Base(c.DatabasePath)
	fmt.Printf("Enter password for %s\033[32m%s\033[0m: ", dir, base)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	fmt.Println()                   // Newline
	c.masterPassword = bytePassword // Already []byte, no conversion needed
	c.passwordSet = true
	return nil
}

// Mkdir creates a group if it doesn't exist (recursive)
func (c *Client) Mkdir(groupPath string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	// Clean path
	groupPath = filepath.ToSlash(filepath.Clean(groupPath))

	// FAST PATH: If the final group already exists, do nothing (1 process call total)
	if c.GroupExists(groupPath) {
		return nil
	}

	// It doesn't exist, we must build it iteratively.
	// Optimize by finding the deepest existing parent to avoid calling GroupExists()
	// on the root paths repeatedly.
	parts := strings.Split(groupPath, "/")

	// Find the deepest existing parent by working backwards
	startIndex := 0
	for i := len(parts) - 1; i > 0; i-- {
		parentPath := strings.Join(parts[:i], "/")
		if parentPath == "" {
			continue
		}
		if c.GroupExists(parentPath) {
			startIndex = i
			break
		}
	}

	// Now create only the missing directories, from startIndex to the end
	currentPath := ""
	if startIndex > 0 {
		currentPath = strings.Join(parts[:startIndex], "/")
	}

	for i := startIndex; i < len(parts); i++ {
		part := parts[i]
		if part == "" {
			continue
		}
		if currentPath != "" {
			currentPath += "/"
		}
		currentPath += part

		out, err := c.runCmd("mkdir", c.DatabasePath, currentPath)
		if err != nil {
			return fmt.Errorf("keepassxc-cli mkdir failed for '%s': %s: %s", currentPath, err, out)
		}
	}
	return nil
}

// VerifyConnection runs a simple command against the database root to confirm
// the master password is correct and the database is accessible.
func (c *Client) VerifyConnection() error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}
	_, err := c.runCmd("ls", "-q", c.DatabasePath)
	return err
}

// GroupExists checks if a group exists.
// Returns false both when the group is absent and when the check itself fails.
func (c *Client) GroupExists(path string) bool {
	if err := c.EnsureUnlocked(); err != nil {
		return false
	}

	path = filepath.ToSlash(filepath.Clean(path))

	// 'ls' exits 0 if the group exists, non-zero otherwise.
	_, err := c.runCmd("ls", "-q", c.DatabasePath, path)
	return err == nil
}

// Search queries KeePassXC for entries matching a string and returns their paths
func (c *Client) Search(query string) ([]string, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return nil, err
	}

	out, err := c.runCmdQuiet("search", c.DatabasePath, query)
	if err != nil {
		// keepassxc-cli returns exit status 1 when no records are found,
		// but also when the master password is wrong.
		// If runCmdQuiet found actual error text on stderr, it appended it
		// (e.g. "exit status 1: Invalid credentials").
		if strings.Contains(err.Error(), ": ") {
			return nil, fmt.Errorf("KeePassXC error: %w", err)
		}
		// Otherwise, it just means "not found" (silent exit 1)
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var results []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Strip leading slash — keepassxc-cli ls/search sometimes emits absolute paths
		line = strings.TrimPrefix(line, "/")
		// keepassxc-cli search sometimes outputs "Database unlocked" or other info, filter them
		if line != "" && !strings.Contains(strings.ToLower(line), "database") {
			results = append(results, line)
		}
	}

	return results, nil
}

// AddEntry adds a new entry to KeePassXC.
// It writes three lines to stdin: master password, entry password, entry password (confirm).
// This does not use runCmd because it requires a custom stdin sequence.
func (c *Client) AddEntry(group, title string, password []byte, username string, url string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	if err := c.Mkdir(group); err != nil {
		return fmt.Errorf("failed to create KeePass group '%s': %w", group, err)
	}

	fullPath := group
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += title

	// keepassxc uses forward slashes
	fullPath = filepath.ToSlash(filepath.Clean(fullPath))

	cmdAdd := buildCmd("add", c.DatabasePath, fullPath,
		"--username", username,
		"--url", url,
		"-p",
	)

	var outBuf bytes.Buffer
	cmdAdd.Stdout = &outBuf
	cmdAdd.Stderr = &outBuf

	stdin, err := cmdAdd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmdAdd.Start(); err != nil {
		return err
	}

	_, _ = stdin.Write(c.getMasterPassword())
	_, _ = stdin.Write([]byte("\n"))
	_, _ = stdin.Write(password)
	_, _ = stdin.Write([]byte("\n"))
	_, _ = stdin.Write(password)
	_, _ = stdin.Write([]byte("\n"))
	_ = stdin.Close()

	if err := cmdAdd.Wait(); err != nil {
		return fmt.Errorf("keepassxc-cli add failed: %s: %s", err, outBuf.String())
	}

	return nil
}

// DeleteEntry removes an entry from KeePassXC
func (c *Client) DeleteEntry(entryPath string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	out, err := c.runCmd("rm", c.DatabasePath, entryPath)
	if err != nil {
		return fmt.Errorf("keepassxc-cli rm failed: %s: %s", err, out)
	}
	return nil
}

// UpdateEntryUsername updates the Username field (last known path) of an entry.
func (c *Client) UpdateEntryUsername(entryPath, username string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	out, err := c.runCmd("edit", "--username", username, c.DatabasePath, entryPath)
	if err != nil {
		return fmt.Errorf("keepassxc-cli edit failed: %s: %s", err, out)
	}
	return nil
}

// GetPassword retrieves a password for an entry
func (c *Client) GetPassword(entryPath string) ([]byte, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return nil, err
	}

	out, err := c.runCmdQuiet("show", "-s", "-a", "password", "-q", c.DatabasePath, entryPath)
	if err != nil {
		if strings.Contains(err.Error(), ": ") {
			// Actually failed with a real error string
			return nil, fmt.Errorf("failed to get password: %w", err)
		}
		// Otherwise, silent "not found"
		return nil, fmt.Errorf("entry not found: %s", entryPath)
	}

	password := bytes.TrimSpace(out)

	// Create copy to own memory
	passCopy := make([]byte, len(password))
	copy(passCopy, password)

	// Zero out the original bytes
	for i := range out {
		out[i] = 0
	}

	return passCopy, nil
}

// GetAttribute retrieves a single named attribute from a KeePass entry.
// Common attribute names: "Username", "URL", "Notes".
func (c *Client) GetAttribute(entryPath, attribute string) (string, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return "", err
	}

	out, err := c.runCmdQuiet("show", "-a", attribute, "-q", c.DatabasePath, entryPath)
	if err != nil {
		return "", fmt.Errorf("failed to get attribute '%s': %w", attribute, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// ListEntries returns the entry names (titles) directly under a group.
// Sub-groups (lines ending with "/") are excluded from the result.
func (c *Client) ListEntries(group string) ([]string, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return nil, err
	}

	out, err := c.runCmdQuiet("ls", "-q", "-f", c.DatabasePath, group)
	if err != nil {
		return nil, fmt.Errorf("keepassxc-cli ls failed for group '%s': %w", group, err)
	}

	var entries []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Sub-groups end with "/", skip them
		if strings.HasSuffix(line, "/") {
			continue
		}
		// Skip keepassxc-cli info lines
		if strings.Contains(strings.ToLower(line), "database") {
			continue
		}
		entries = append(entries, line)
	}

	return entries, nil
}

// EditEntryTitle atomically updates the title and username of a KeePass entry
// in a single keepassxc-cli edit call. This is used by orphan recovery to
// relink a renamed archive to its existing entry.
func (c *Client) EditEntryTitle(entryPath, newTitle, newUsername string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	out, err := c.runCmd("edit", "--title", newTitle, "--username", newUsername, c.DatabasePath, entryPath)
	if err != nil {
		return fmt.Errorf("keepassxc-cli edit failed: %s: %s", err, out)
	}
	return nil
}

// UpdateEntryNotes updates the Notes field of an existing entry.
func (c *Client) UpdateEntryNotes(entryPath, notes string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	out, err := c.runCmd("edit", "--notes", notes, c.DatabasePath, entryPath)
	if err != nil {
		return fmt.Errorf("keepassxc-cli edit --notes failed: %s: %s", err, out)
	}
	return nil
}
