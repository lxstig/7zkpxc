package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lxstig/7zkpxc/internal/config"
)

// createDummy7zForOrphan creates a small real 7z archive to satisfy VerifyPassword
// during brute-force testing.
func createDummy7zForOrphan(t *testing.T, dir, name string, password []byte) string {
	t.Helper()
	if _, err := exec.LookPath("7z"); err != nil {
		t.Skip("7z not installed")
	}

	srcFile := filepath.Join(dir, "data.txt")
	_ = os.WriteFile(srcFile, []byte("test"), 0644)

	archivePath := filepath.Join(dir, name)
	cmd := exec.Command("7z", "a", "-p"+string(password), "-mhe=on", archivePath, srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("7z create failed: %v\n%s", err, out)
	}

	return archivePath
}

func TestPromptOrphanRecovery_Single_Accept(t *testing.T) {
	orphans := []OrphanCandidate{
		{EntryPath: "Grp/test (abcd)", Title: "test (abcd)", LastKnownPath: "/old/path"},
	}

	// Mock stdin
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin; _ = r.Close() }()

	go func() {
		_, _ = w.Write([]byte("y\n"))
		_ = w.Close()
	}()

	chosen, err := promptOrphanRecovery(orphans, "/new/path")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if chosen.EntryPath != orphans[0].EntryPath {
		t.Errorf("wrong candidate chosen: %v", chosen)
	}
}

func TestPromptOrphanRecovery_Single_Decline(t *testing.T) {
	orphans := []OrphanCandidate{
		{EntryPath: "Grp/test (abcd)", Title: "test (abcd)", LastKnownPath: "/old/path"},
	}

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin; _ = r.Close() }()

	go func() {
		_, _ = w.Write([]byte("n\n"))
		_ = w.Close()
	}()

	_, err := promptOrphanRecovery(orphans, "/new/path")
	if err == nil || !strings.Contains(err.Error(), "declined") {
		t.Fatalf("expected declined error, got %v", err)
	}
}

func TestPromptOrphanRecovery_Multiple_Select(t *testing.T) {
	orphans := []OrphanCandidate{
		{EntryPath: "Grp/1 (1111)", Title: "1", LastKnownPath: "/old/1"},
		{EntryPath: "Grp/2 (2222)", Title: "2", LastKnownPath: "/old/2"},
	}

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin; _ = r.Close() }()

	go func() {
		_, _ = w.Write([]byte("2\n"))
		_ = w.Close()
	}()

	time.Sleep(50 * time.Millisecond) // Give scanner time

	chosen, err := promptOrphanRecovery(orphans, "/new/path")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if chosen.EntryPath != orphans[1].EntryPath {
		t.Errorf("wrong candidate chosen: %v", chosen)
	}
}

func TestPromptOrphanRecovery_Multiple_Bruteforce(t *testing.T) {
	orphans := []OrphanCandidate{
		{EntryPath: "Grp/1 (1111)"},
		{EntryPath: "Grp/2 (2222)"},
	}

	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin; _ = r.Close() }()

	go func() {
		_, _ = w.Write([]byte("B\n"))
		_ = w.Close()
	}()

	_, err := promptOrphanRecovery(orphans, "/new/path")
	if !errors.Is(err, errBruteforceRequested) {
		t.Fatalf("expected bruteforce requested, got %v", err)
	}
}

func TestBruteforceOrphans_Success(t *testing.T) {
	tmpDir := t.TempDir()
	pw := []byte("correct_password")
	archivePath := createDummy7zForOrphan(t, tmpDir, "brute.7z", pw)

	cfg := &config.Config{
		SevenZip: config.SevenZipConfig{BinaryPath: "7z"},
	}

	mockKP := NewMockPasswordProvider()
	mockKP.SetPassword("Grp/1 (1111)", []byte("wrong_password"))
	mockKP.SetPassword("Grp/2 (2222)", pw)
	mockKP.SetPassword("Grp/3 (3333)", []byte("another_wrong"))

	orphans := []OrphanCandidate{
		{EntryPath: "Grp/1 (1111)"},
		{EntryPath: "Grp/2 (2222)", Title: "brute (2222)", LastKnownPath: "/old"},
		{EntryPath: "Grp/3 (3333)"},
	}

	// bruteforceOrphans internally calls relinkOrphanEntry if match found
	// Since EditEntryTitle is called on the real client, we mocked it on MockPasswordProvider?
	// Wait, relinkOrphanEntry expects PasswordProvider to have EditEntryTitle. Let's check if MockPasswordProvider has it.

	// Disable stdout printing nicely
	origStdout := os.Stdout
	nullFile, _ := os.Open(os.DevNull)
	os.Stdout = nullFile
	defer func() {
		os.Stdout = origStdout
		_ = nullFile.Close()
	}()

	recoveredPw, newPath, err := bruteforceOrphans(mockKP, cfg, archivePath, orphans)
	if err != nil {
		t.Fatalf("bruteforce failed: %v", err)
	}
	if string(recoveredPw) != string(pw) {
		t.Errorf("recovered password wrong: %s", recoveredPw)
	}
	if newPath == "" {
		t.Errorf("newPath empty")
	}

	// Verify relink was called
	calls := mockKP.GetCalls()
	foundRelink := false
	for _, call := range calls {
		if strings.HasPrefix(call, "edit-entry:Grp/2 (2222)") {
			foundRelink = true
		}
	}
	if !foundRelink {
		t.Errorf("Missing EditEntryTitle call to relink entry")
	}
}

func TestVerifyAndRelinkOrphan_Success(t *testing.T) {
	tmpDir := t.TempDir()
	pw := []byte("correct_password")
	archivePath := createDummy7zForOrphan(t, tmpDir, "verify.7z", pw)

	cfg := &config.Config{
		SevenZip: config.SevenZipConfig{BinaryPath: "7z"},
	}

	mockKP := NewMockPasswordProvider()
	mockKP.SetPassword("Grp/chosen (1234)", pw)

	candidate := OrphanCandidate{
		EntryPath: "Grp/chosen (1234)",
		Title:     "chosen (1234)",
	}

	origStdout := os.Stdout
	nullFile, _ := os.Open(os.DevNull)
	os.Stdout = nullFile
	defer func() {
		os.Stdout = origStdout
		_ = nullFile.Close()
	}()

	recoveredPw, newPath, err := verifyAndRelinkOrphan(mockKP, cfg, archivePath, candidate)
	if err != nil {
		t.Fatalf("verifyAndRelinkOrphan failed: %v", err)
	}
	if string(recoveredPw) != string(pw) {
		t.Errorf("wrong password: %s", recoveredPw)
	}
	if newPath == "" {
		t.Errorf("newPath empty")
	}
}

func TestAttemptOrphanRecovery_NoOrphans(t *testing.T) {
	cfg := &config.Config{
		General: config.GeneralConfig{DefaultGroup: "Grp"},
	}
	mockKP := NewMockPasswordProvider()
	// No passwords set, findOrphansInGroup will return 0 orphans

	_, _, err := attemptOrphanRecovery(mockKP, cfg, "/fake/path")
	if err == nil || !strings.Contains(err.Error(), "no orphan entries") {
		t.Fatalf("expected no orphans error, got %v", err)
	}
}
