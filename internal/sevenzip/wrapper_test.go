package sevenzip

import (
	"fmt"
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
	err = Run("7z", []byte("testpassword"), []string{"invalidcmd"})
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
	password := []byte("SecretPass123!")

	// 1. Create dummy content
	content := []byte("This is a secret message.")
	if err := os.WriteFile(sourceFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Encrypt using Run() (Mocking '7zkpxc a')
	// We pass -p to trigger 7z's password prompt.
	// wrapper.Run should handle the prompt interaction.
	err := Run("7z", password, []string{"a", archiveFile, sourceFile, "-p", "-mhe=on"})
	if err != nil {
		t.Fatalf("Failed to encrypt archive: %v", err)
	}

	// 3. Verify archive exists
	if _, err := os.Stat(archiveFile); os.IsNotExist(err) {
		t.Fatal("Archive file was not created")
	}

	// 4. Extract using Run() (Mocking '7zkpxc x')
	// We pass -y to overwrite if needed (though dir is unique)
	err = Run("7z", password, []string{"x", archiveFile, "-o" + extractDir, "-y"})
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

// -------------------------------------------------------------------
// sevenZipExitCodeDesc
// -------------------------------------------------------------------

func TestSevenZipExitCodeDesc(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "No error"},
		{1, "Warning (Non fatal error(s)). For example, one or more files were locked by some other application, so they were not compressed."},
		{2, "Fatal error"},
		{7, "Command line error"},
		{8, "Not enough memory for operation"},
		{255, "User stopped the process"},
		{42, "Unknown error"},
		{-1, "Unknown error"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			got := sevenZipExitCodeDesc(tt.code)
			if got != tt.want {
				t.Errorf("sevenZipExitCodeDesc(%d) = %q, want %q", tt.code, got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------
// VerifyPassword
// -------------------------------------------------------------------

func TestVerifyPassword_EncryptedArchive(t *testing.T) {
	if _, err := exec.LookPath("7z"); err != nil {
		t.Skip("7z not installed, skipping")
	}

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "secret.txt")
	archiveFile := filepath.Join(tmpDir, "encrypted.7z")
	password := []byte("CorrectPassword123!")

	if err := os.WriteFile(sourceFile, []byte("secret content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create encrypted archive
	if err := Run("7z", password, []string{"a", archiveFile, sourceFile, "-p", "-mhe=on"}); err != nil {
		t.Fatalf("Failed to create encrypted archive: %v", err)
	}

	// Correct password → MatchCorrect
	match, err := VerifyPassword("7z", password, archiveFile)
	if err != nil {
		t.Fatalf("VerifyPassword with correct password failed: %v", err)
	}
	if match != MatchCorrect {
		t.Errorf("expected MatchCorrect, got %v", match)
	}
}

func TestVerifyPassword_WrongPassword(t *testing.T) {
	if _, err := exec.LookPath("7z"); err != nil {
		t.Skip("7z not installed, skipping")
	}

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "secret.txt")
	archiveFile := filepath.Join(tmpDir, "encrypted.7z")
	password := []byte("CorrectPassword123!")

	if err := os.WriteFile(sourceFile, []byte("secret content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Run("7z", password, []string{"a", archiveFile, sourceFile, "-p", "-mhe=on"}); err != nil {
		t.Fatalf("Failed to create encrypted archive: %v", err)
	}

	// Wrong password → MatchFailed
	match, err := VerifyPassword("7z", []byte("WrongPassword!"), archiveFile)
	if match != MatchFailed {
		t.Errorf("expected MatchFailed, got %v", match)
	}
	if err == nil {
		t.Error("expected error for wrong password")
	}
}

func TestVerifyPassword_UnencryptedArchive(t *testing.T) {
	if _, err := exec.LookPath("7z"); err != nil {
		t.Skip("7z not installed, skipping")
	}

	tmpDir := t.TempDir()
	sourceFile := filepath.Join(tmpDir, "plain.txt")
	archiveFile := filepath.Join(tmpDir, "unencrypted.7z")

	if err := os.WriteFile(sourceFile, []byte("plain content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create unencrypted archive (no -p flag)
	cmd := exec.Command("7z", "a", archiveFile, sourceFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create unencrypted archive: %v", err)
	}

	// Any password against unencrypted archive → MatchUnencrypted
	match, err := VerifyPassword("7z", []byte("anything"), archiveFile)
	if err != nil {
		t.Fatalf("VerifyPassword on unencrypted archive failed: %v", err)
	}
	if match != MatchUnencrypted {
		t.Errorf("expected MatchUnencrypted, got %v", match)
	}
}

func TestVerifyPassword_NonexistentArchive(t *testing.T) {
	if _, err := exec.LookPath("7z"); err != nil {
		t.Skip("7z not installed, skipping")
	}

	match, err := VerifyPassword("7z", []byte("pass"), "/nonexistent/archive.7z")
	if match != MatchFailed {
		t.Errorf("expected MatchFailed for nonexistent file, got %v", match)
	}
	if err == nil {
		t.Error("expected error for nonexistent archive")
	}
}

