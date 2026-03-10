package keepass

import (
	"bytes"
	"fmt"
	"io"
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
	KeyFile        string
	masterPassword []byte // lowercase for encapsulation, zeroed on Close()
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
}

// getMasterPassword returns the master password.
// Be very careful when using this to avoid creating string copies that linger in memory.
func (c *Client) getMasterPassword() []byte {
	return c.masterPassword
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
	if len(c.masterPassword) > 0 {
		return nil
	}

	base := filepath.Base(c.DatabasePath)
	dir := c.DatabasePath[:len(c.DatabasePath)-len(base)]
	fmt.Printf("Enter password for %s\033[32m%s\033[0m: ", dir, base)
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}
	fmt.Println()                   // Newline
	c.masterPassword = bytePassword // Already []byte, no conversion needed
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

		cmd := buildCmd("mkdir", c.DatabasePath, currentPath)
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}

		if err := cmd.Start(); err != nil {
			return err
		}

		_, _ = stdin.Write(c.getMasterPassword())
		_, _ = stdin.Write([]byte("\n"))
		_ = stdin.Close()

		err = cmd.Wait()
		if err != nil {
			return fmt.Errorf("keepassxc-cli mkdir failed for '%s': %s: %s", currentPath, err, outBuf.String())
		}
	}
	return nil
}

// GroupExists checks if a group exists
func (c *Client) GroupExists(path string) bool {
	if err := c.EnsureUnlocked(); err != nil {
		return false
	}

	path = filepath.ToSlash(filepath.Clean(path))

	// 'ls' will succeed (exit code 0) if the group exists, and fail if not.
	cmdLs := buildCmd("ls", "-q", c.DatabasePath, path)
	stdinLs, err := cmdLs.StdinPipe()
	if err == nil {
		if err := cmdLs.Start(); err == nil {
			_, _ = stdinLs.Write(c.getMasterPassword())
			_, _ = stdinLs.Write([]byte("\n"))
			_ = stdinLs.Close()
			if err := cmdLs.Wait(); err == nil {
				return true // It's a group
			}
		}
	}

	return false
}

// Search queries KeePassXC for entries matching a string and returns their paths
func (c *Client) Search(query string) ([]string, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return nil, err
	}

	cmd := buildCmd("search", c.DatabasePath, query)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = io.Discard // suppress "Enter password to unlock" prompt spam

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

	if err := cmd.Wait(); err != nil {
		// keepassxc-cli returns exit status 1 when no records are found or error occurs
		return nil, nil
	}

	lines := strings.Split(strings.TrimSpace(outBuf.String()), "\n")
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

// AddEntry adds a new entry to KeePassXC
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

	cmd := buildCmd("rm", c.DatabasePath, entryPath)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	_, _ = stdin.Write(c.getMasterPassword())
	_, _ = stdin.Write([]byte("\n"))
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("keepassxc-cli rm failed: %s: %s", err, outBuf.String())
	}

	return nil
}

// UpdateEntryUsername updates the Username field (last known path) of an entry.
func (c *Client) UpdateEntryUsername(entryPath, username string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	cmd := buildCmd("edit", "--username", username, c.DatabasePath, entryPath)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	_, _ = stdin.Write(c.getMasterPassword())
	_, _ = stdin.Write([]byte("\n"))
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("keepassxc-cli edit failed: %s: %s", err, outBuf.String())
	}

	return nil
}

// GetPassword retrieves a password for an entry
func (c *Client) GetPassword(entryPath string) ([]byte, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return nil, err
	}

	cmd := buildCmd("show", "-s", "-a", "password", "-q", c.DatabasePath, entryPath)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = io.Discard // suppress "Enter password to unlock" prompt spam

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

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("failed to get password: %w", err)
	}

	password := bytes.TrimSpace(outBuf.Bytes())

	// Create copy to own memory
	passCopy := make([]byte, len(password))
	copy(passCopy, password)

	// Zero out the buffer
	outBytes := outBuf.Bytes()
	for i := range outBytes {
		outBytes[i] = 0
	}

	return passCopy, nil
}

// GetAttribute retrieves a single named attribute from a KeePass entry.
// Common attribute names: "Username", "URL", "Notes".
func (c *Client) GetAttribute(entryPath, attribute string) (string, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return "", err
	}

	cmd := buildCmd("show", "-a", attribute, "-q", c.DatabasePath, entryPath)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = io.Discard // suppress "Enter password to unlock" prompt spam

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	_, _ = stdin.Write(c.getMasterPassword())
	_, _ = stdin.Write([]byte("\n"))
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("failed to get attribute '%s': %w", attribute, err)
	}

	return strings.TrimSpace(outBuf.String()), nil
}
