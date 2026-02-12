package keepass

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

// Close securely wipes the master password from memory
func (c *Client) Close() {
	for i := range c.masterPassword {
		c.masterPassword[i] = 0
	}
	c.masterPassword = nil
}

// getMasterPassword returns the password as string for CLI usage
func (c *Client) getMasterPassword() string {
	return string(c.masterPassword)
}

// GeneratePassword creates a secure random password using keepassxc-cli generate.
// This delegates all cryptographic work to KeePassXC's audited generator.
// Flags: -L length, -l lowercase, -U uppercase, -n numbers, -s special characters.
func (c *Client) GeneratePassword(length int) (string, error) {
	cmd := exec.Command("keepassxc-cli", "generate",
		"-L", strconv.Itoa(length),
		"-l", "-U", "-n", "-s",
	)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("keepassxc-cli generate failed: %w: %s", err, errBuf.String())
	}

	password := strings.TrimSpace(outBuf.String())
	if password == "" {
		return "", fmt.Errorf("keepassxc-cli generate returned empty password")
	}

	return password, nil
}

// EnsureUnlocked prompts for master password if not set
func (c *Client) EnsureUnlocked() error {
	if len(c.masterPassword) > 0 {
		return nil
	}

	fmt.Printf("Enter password for %s: ", c.DatabasePath)
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

	parts := strings.Split(groupPath, "/")
	currentPath := ""

	for _, part := range parts {
		if part == "" {
			continue
		}
		if currentPath != "" {
			currentPath += "/"
		}
		currentPath += part

		cmd := exec.Command("keepassxc-cli", "mkdir", c.DatabasePath, currentPath)

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

		_, _ = fmt.Fprintf(stdin, "%s\n", c.getMasterPassword())
		_ = stdin.Close()

		_ = cmd.Wait()
	}
	return nil
}

// Exists checks if an entry (or group) exists
func (c *Client) Exists(path string) bool {
	if err := c.EnsureUnlocked(); err != nil {
		return false
	}

	cmd := exec.Command("keepassxc-cli", "show", "-q", c.DatabasePath, path)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false
	}

	if err := cmd.Start(); err != nil {
		return false
	}

	_, _ = fmt.Fprintf(stdin, "%s\n", c.getMasterPassword())
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return false
	}
	return true
}

// AddEntry adds a new entry to KeePassXC
func (c *Client) AddEntry(group, title, password, specificUrl string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	_ = c.Mkdir(group)

	fullPath := group
	if !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += title

	// Check existence before add to avoid messy errors
	if c.Exists(fullPath) {
		return fmt.Errorf("entry '%s' already exists", fullPath)
	}

	cmd := exec.Command("keepassxc-cli", "add", c.DatabasePath, fullPath,
		"--username", "7zkpxc",
		"--url", specificUrl,
		"-p",
	)

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

	_, _ = fmt.Fprintf(stdin, "%s\n", c.getMasterPassword())
	_, _ = fmt.Fprintf(stdin, "%s\n", password)
	_, _ = fmt.Fprintf(stdin, "%s\n", password)

	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("keepassxc-cli failed: %s: %s", err, outBuf.String())
	}

	return nil
}

// DeleteEntry removes an entry from KeePassXC
func (c *Client) DeleteEntry(entryPath string) error {
	if err := c.EnsureUnlocked(); err != nil {
		return err
	}

	cmd := exec.Command("keepassxc-cli", "rm", c.DatabasePath, entryPath)

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

	_, _ = fmt.Fprintf(stdin, "%s\n", c.getMasterPassword())
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("keepassxc-cli rm failed: %s: %s", err, outBuf.String())
	}

	return nil
}

// GetPassword retrieves a password for an entry
func (c *Client) GetPassword(entryPath string) (string, error) {
	if err := c.EnsureUnlocked(); err != nil {
		return "", err
	}

	cmd := exec.Command("keepassxc-cli", "show", "-s", "-a", "password", "-q", c.DatabasePath, entryPath)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	_, _ = fmt.Fprintf(stdin, "%s\n", c.getMasterPassword())
	_ = stdin.Close()

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("failed to get password: %w", err)
	}

	return strings.TrimSpace(outBuf.String()), nil
}
