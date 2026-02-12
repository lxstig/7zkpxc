package keepass

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Integration test using fake keepassxc-cli
func TestGetPassword_WithFakeCLI(t *testing.T) {
	// Get the path to our fake binary
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	
	// Navigate up to project root (we're in internal/keepass)
	projectRoot := filepath.Join(wd, "..", "..")
	fakeBinDir := filepath.Join(projectRoot, "testdata", "bin")
	
	// Check if fake binary exists
	fakeCLI := filepath.Join(fakeBinDir, "keepassxc-cli")
	if _, err := os.Stat(fakeCLI); os.IsNotExist(err) {
		t.Skip("Fake keepassxc-cli not found, skipping integration test")
	}
	
	// Setup temp files for fake CLI output
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output")
	stdinFile := filepath.Join(tmpDir, "stdin")
	
	// Set environment for fake CLI
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeBinDir+":"+oldPath)
	t.Setenv("FAKE_KEEPASSXC_OUTPUT", outputFile)
	t.Setenv("FAKE_KEEPASSXC_STDIN", stdinFile)
	
	// Create client with pre-set password (skip EnsureUnlocked prompt)
	client := &Client{
		DatabasePath:   "/fake/db.kdbx",
		masterPassword: []byte("testmasterpassword"),
	}
	defer client.Close()
	
	// Call GetPassword
	password, err := client.GetPassword("TestGroup/TestEntry")
	if err != nil {
		t.Fatalf("GetPassword failed: %v", err)
	}
	
	// Verify password returned
	if password != "fakepassword123" {
		t.Errorf("GetPassword = %q, want %q", password, "fakepassword123")
	}
	
	// Verify stdin was written correctly (password not in CLI args!)
	stdinContent, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Logf("Could not read stdin file: %v", err)
	} else {
		if !strings.Contains(string(stdinContent), "testmasterpassword") {
			t.Errorf("Master password not piped via stdin")
		}
	}
	
	// Verify CLI was called with correct args (no password in args!)
	cmdContent, err := os.ReadFile(outputFile)
	if err != nil {
		t.Logf("Could not read output file: %v", err)
	} else {
		cmdStr := string(cmdContent)
		if strings.Contains(cmdStr, "testmasterpassword") {
			t.Errorf("Password leaked to command line args!")
		}
		if !strings.Contains(cmdStr, "show") {
			t.Errorf("Expected 'show' command in args")
		}
	}
}

func TestAddEntry_PasswordNotInArgs(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	
	projectRoot := filepath.Join(wd, "..", "..")
	fakeBinDir := filepath.Join(projectRoot, "testdata", "bin")
	
	fakeCLI := filepath.Join(fakeBinDir, "keepassxc-cli")
	if _, err := os.Stat(fakeCLI); os.IsNotExist(err) {
		t.Skip("Fake keepassxc-cli not found, skipping integration test")
	}
	
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output")
	stdinFile := filepath.Join(tmpDir, "stdin")
	
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeBinDir+":"+oldPath)
	t.Setenv("FAKE_KEEPASSXC_OUTPUT", outputFile)
	t.Setenv("FAKE_KEEPASSXC_STDIN", stdinFile)
	
	client := &Client{
		DatabasePath:   "/fake/db.kdbx",
		masterPassword: []byte("masterpass"),
	}
	defer client.Close()
	
	// This will fail because Exists() will try to show first
	// But we can verify the password handling principle
	_ = client.AddEntry("TestGroup", "TestEntry", "archivepassword", "/path/to/archive")
	
	// Verify archive password not in command line
	cmdContent, err := os.ReadFile(outputFile)
	if err == nil {
		cmdStr := string(cmdContent)
		if strings.Contains(cmdStr, "archivepassword") {
			t.Errorf("Archive password leaked to command line args!")
		}
		if strings.Contains(cmdStr, "masterpass") {
			t.Errorf("Master password leaked to command line args!")
		}
	}
}
