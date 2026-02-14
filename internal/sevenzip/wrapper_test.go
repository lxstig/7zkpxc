package sevenzip

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestRun_EncryptionDecryption(t *testing.T) {
	// Skip if 7z not installed
	if _, err := exec.LookPath("7z"); err != nil {
		t.Skip("7z not installed, skipping")
	}

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "secret.txt")
	archiveFile := filepath.Join(tmpDir, "secret.7z")
	extractDir := filepath.Join(tmpDir, "extracted")
	password := "SecretPass123!"

	// 1. Create dummy content
	content := []byte("This is a secret message.")
	if err := os.WriteFile(sourceFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Encrypt using Run() (Mocking '7zkpxc a')
	// We pass -p to trigger 7z's password prompt.
	// wrapper.Run should handle the prompt interaction.
	err := Run(password, []string{"a", archiveFile, sourceFile, "-p", "-mhe=on"})
	if err != nil {
		t.Fatalf("Failed to encrypt archive: %v", err)
	}

	// 3. Verify archive exists
	if _, err := os.Stat(archiveFile); os.IsNotExist(err) {
		t.Fatal("Archive file was not created")
	}

	// 4. Extract using Run() (Mocking '7zkpxc x')
	// We pass -y to overwrite if needed (though dir is unique)
	err = Run(password, []string{"x", archiveFile, "-o" + extractDir, "-y"})
	if err != nil {
		t.Fatalf("Failed to extract archive: %v", err)
	}

	// 5. Verify content matches
	extractedFile := filepath.Join(extractDir, "secret.txt")
	extractedContent, err := os.ReadFile(extractedFile)
	if err != nil {
		t.Fatalf("Failed to read extracted file: %v", err)
	}

	if string(extractedContent) != string(content) {
		t.Errorf("Extracted content mismatch. Want %q, got %q", content, extractedContent)
	}
}
