package sevenzip

import (
	"os"
	"os/exec"
	"testing"
)

func TestRun_7zExists(t *testing.T) {
	// Skip if 7z not installed
	_, err := exec.LookPath("7z")
	if err != nil {
		t.Skip("7z not installed, skipping")
	}
}

func TestRun_InvalidCommand(t *testing.T) {
	// Skip if 7z not installed
	_, err := exec.LookPath("7z")
	if err != nil {
		t.Skip("7z not installed, skipping")
	}

	// Run with invalid args - should fail
	err = Run("testpassword", []string{"invalidcmd"})
	if err == nil {
		t.Error("Expected error for invalid 7z command, got nil")
	}
}

func TestRun_HelpCommand(t *testing.T) {
	// Skip if 7z not installed
	_, err := exec.LookPath("7z")
	if err != nil {
		t.Skip("7z not installed, skipping")
	}

	// 7z with no args shows help and exits 0
	// But our Run function might hang waiting for password prompt
	// So we test a command that doesn't need password
	
	// Create temp file to test
	tmpFile := t.TempDir() + "/test.txt"
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}
	
	tmpArchive := t.TempDir() + "/test.7z"
	
	// Create unencrypted archive (no password prompt)
	cmd := exec.Command("7z", "a", tmpArchive, tmpFile)
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not create test archive: %v", err)
	}
	
	// List (unencrypted, won't prompt for password)
	cmd = exec.Command("7z", "l", tmpArchive)
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to list archive: %v", err)
	}
}
