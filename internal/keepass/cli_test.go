package keepass

import (
	"os/exec"
	"strings"
	"testing"
)

func TestMasterPasswordZeroedOnClose(t *testing.T) {
	c := &Client{
		masterPassword: []byte("supersecret123!@#"),
	}

	// Verify password is set
	if len(c.masterPassword) == 0 {
		t.Fatal("password should be set before Close()")
	}

	// Close should zero the bytes
	c.Close()

	// Verify password is nil
	if c.masterPassword != nil {
		t.Fatal("masterPassword should be nil after Close()")
	}
}

func TestMasterPasswordBytesZeroed(t *testing.T) {
	// Store reference to prove bytes were zeroed in-place
	secret := []byte("supersecret123!@#")
	c := &Client{
		masterPassword: secret,
	}

	c.Close()

	// Verify original bytes were zeroed, not just the reference
	for i, b := range secret {
		if b != 0 {
			t.Fatalf("byte at index %d not zeroed: got %d", i, b)
		}
	}
}

func TestGetMasterPassword(t *testing.T) {
	c := &Client{
		masterPassword: []byte("testpass"),
	}

	got := string(c.getMasterPassword())
	if got != "testpass" {
		t.Errorf("getMasterPassword() = %q, want %q", got, "testpass")
	}
}

func TestGeneratePassword_RequiresKeepassxcCLI(t *testing.T) {
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed, skipping")
	}

	client := New("/dummy/path.kdbx")

	pw, err := client.GeneratePassword(64)
	if err != nil {
		t.Fatalf("GeneratePassword(64) failed: %v", err)
	}

	if len(pw) != 64 {
		t.Errorf("GeneratePassword(64) returned length %d, want 64", len(pw))
	}
}

func TestGeneratePassword_Lengths(t *testing.T) {
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed, skipping")
	}

	client := New("/dummy/path.kdbx")

	tests := []struct {
		name   string
		length int
	}{
		{"Minimum (32)", 32},
		{"Default (64)", 64},
		{"Maximum (128)", 128},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pw, err := client.GeneratePassword(tt.length)
			if err != nil {
				t.Fatalf("GeneratePassword(%d) failed: %v", tt.length, err)
			}

			if len(pw) != tt.length {
				t.Errorf("len = %d, want %d", len(pw), tt.length)
			}

			if len(pw) == 0 {
				t.Error("password is empty")
			}
		})
	}
}

func TestGeneratePassword_Uniqueness(t *testing.T) {
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed, skipping")
	}

	client := New("/dummy/path.kdbx")

	passwords := make(map[string]struct{})
	for i := 0; i < 10; i++ {
		pw, err := client.GeneratePassword(64)
		if err != nil {
			t.Fatalf("GeneratePassword failed on iteration %d: %v", i, err)
		}
		if _, exists := passwords[string(pw)]; exists {
			t.Fatalf("duplicate password generated on iteration %d", i)
		}
		passwords[string(pw)] = struct{}{}
	}
}

func TestNew_Defaults(t *testing.T) {
	client := New("/test/db.kdbx")
	if client.DatabasePath != "/test/db.kdbx" {
		t.Errorf("DatabasePath = %q, want %q", client.DatabasePath, "/test/db.kdbx")
	}
	if client.masterPassword != nil {
		t.Error("masterPassword should be nil on new client")
	}
}

func TestClearMasterPassword(t *testing.T) {
	secret := []byte("my-secret-password")
	c := &Client{
		masterPassword: secret,
	}

	c.clearMasterPassword()

	// Verify bytes were zeroed in-place
	for i, b := range secret {
		if b != 0 {
			t.Fatalf("byte at index %d not zeroed: got %d", i, b)
		}
	}
	if c.masterPassword != nil {
		t.Error("masterPassword should be nil after clearMasterPassword")
	}
}

func TestClearMasterPassword_AlreadyNil(t *testing.T) {
	c := &Client{masterPassword: nil}
	// Should not panic
	c.clearMasterPassword()
}

func TestBuildCmd(t *testing.T) {
	cmd := buildCmd("show", "--quiet", "/db.kdbx", "entry")
	args := cmd.Args
	// First arg is the binary name
	if args[0] != "keepassxc-cli" {
		t.Errorf("args[0] = %q, want \"keepassxc-cli\"", args[0])
	}
	// Should contain all passed args
	found := false
	for _, a := range args {
		if a == "--quiet" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --quiet in args: %v", args)
	}
}

// -------------------------------------------------------------------
// parseKeepassxcStderr
// -------------------------------------------------------------------

func TestParseKeepassxcStderr_Empty(t *testing.T) {
	result := parseKeepassxcStderr("", "/test/db.kdbx")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseKeepassxcStderr_OnlyPrompt(t *testing.T) {
	// The prompt line ends with ":" and contains the db basename → should be filtered
	result := parseKeepassxcStderr("Enter password to unlock db.kdbx:", "/test/db.kdbx")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseKeepassxcStderr_SearchMiss(t *testing.T) {
	result := parseKeepassxcStderr("No results for that search term.", "/test/db.kdbx")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseKeepassxcStderr_EntryNotFound(t *testing.T) {
	result := parseKeepassxcStderr("Entry not found.", "/test/db.kdbx")
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestParseKeepassxcStderr_RealError(t *testing.T) {
	// Real error text should NOT be filtered
	result := parseKeepassxcStderr("Invalid credentials were provided", "/test/db.kdbx")
	if result != "Invalid credentials were provided" {
		t.Errorf("expected error to pass through, got %q", result)
	}
}

func TestParseKeepassxcStderr_MixedOutput(t *testing.T) {
	input := "Enter password to unlock db.kdbx:\nInvalid credentials were provided\nNo results for that search term."
	result := parseKeepassxcStderr(input, "/test/db.kdbx")
	if result != "Invalid credentials were provided" {
		t.Errorf("expected only real error, got %q", result)
	}
}

func TestParseKeepassxcStderr_MultipleErrors(t *testing.T) {
	input := "Error line 1\nError line 2"
	result := parseKeepassxcStderr(input, "/test/db.kdbx")
	if result != "Error line 1\nError line 2" {
		t.Errorf("expected both error lines, got %q", result)
	}
}

func TestParseKeepassxcStderr_WhitespaceOnly(t *testing.T) {
	result := parseKeepassxcStderr("   \n  \n\n  ", "/test/db.kdbx")
	if result != "" {
		t.Errorf("expected empty string for whitespace-only input, got %q", result)
	}
}

// -------------------------------------------------------------------
// Lock detection pattern tests
// -------------------------------------------------------------------

func TestLockDetection_LockedBy(t *testing.T) {
	errStr := "Database locked by another process"
	lower := strings.ToLower(errStr)
	if !strings.Contains(lower, "locked by") {
		t.Error("'locked by' pattern should match")
	}
}

func TestLockDetection_LockFile(t *testing.T) {
	errStr := "Lock file is present"
	lower := strings.ToLower(errStr)
	if !strings.Contains(lower, "lock file") {
		t.Error("'lock file' pattern should match")
	}
}

func TestLockDetection_DatabaseIsLocked(t *testing.T) {
	errStr := "Database is locked"
	lower := strings.ToLower(errStr)
	if !strings.Contains(lower, "database is locked") {
		t.Error("'database is locked' pattern should match")
	}
}

func TestLockDetection_UnlockDoesNotMatch(t *testing.T) {
	// This was the bug: "Enter password to unlock" should NOT trigger lock detection
	errStr := "Enter password to unlock /path/to/db.kdbx:"
	lower := strings.ToLower(errStr)
	isLocked := strings.Contains(lower, "locked by") ||
		strings.Contains(lower, "lock file") ||
		strings.Contains(lower, "database is locked")
	if isLocked {
		t.Error("'unlock' in prompt should NOT trigger lock detection")
	}
}

// -------------------------------------------------------------------
// clearMasterPassword state
// -------------------------------------------------------------------

func TestClearMasterPassword_ResetsPasswordSet(t *testing.T) {
	c := &Client{
		masterPassword: []byte("secret"),
		passwordSet:    true,
	}

	c.clearMasterPassword()

	if c.passwordSet {
		t.Error("passwordSet should be false after clearMasterPassword")
	}
	if c.masterPassword != nil {
		t.Error("masterPassword should be nil after clearMasterPassword")
	}
}

// -------------------------------------------------------------------
// buildCmd — locale enforcement
// -------------------------------------------------------------------

func TestBuildCmd_EnforcesLocale(t *testing.T) {
	cmd := buildCmd("show", "/db.kdbx", "entry")

	hasLCAll := false
	hasLanguage := false
	hasLang := false
	for _, env := range cmd.Env {
		switch env {
		case "LC_ALL=C":
			hasLCAll = true
		case "LANGUAGE=en_US.UTF-8":
			hasLanguage = true
		case "LANG=en_US.UTF-8":
			hasLang = true
		}
	}

	if !hasLCAll {
		t.Error("buildCmd should set LC_ALL=C")
	}
	if !hasLanguage {
		t.Error("buildCmd should set LANGUAGE=en_US.UTF-8")
	}
	if !hasLang {
		t.Error("buildCmd should set LANG=en_US.UTF-8")
	}
}

func TestBuildCmd_InheritsOSEnv(t *testing.T) {
	cmd := buildCmd("ls", "/db.kdbx")

	// Should inherit OS environment (env length > 3 locale vars)
	if len(cmd.Env) < 3 {
		t.Errorf("buildCmd Env should contain OS env plus locale vars, got %d items", len(cmd.Env))
	}
}

// -------------------------------------------------------------------
// parseKeepassxcStderr — additional edge cases
// -------------------------------------------------------------------

func TestParseKeepassxcStderr_PromptWithDifferentDB(t *testing.T) {
	// Prompt for a different DB file should NOT be filtered
	result := parseKeepassxcStderr("Enter password to unlock other.kdbx:", "/test/db.kdbx")
	if result != "Enter password to unlock other.kdbx:" {
		t.Errorf("prompt for different DB should not be filtered, got %q", result)
	}
}

func TestParseKeepassxcStderr_AllFilteredTypes(t *testing.T) {
	input := "Enter password to unlock db.kdbx:\nNo results for that search term.\nEntry not found.\n  \n"
	result := parseKeepassxcStderr(input, "/test/db.kdbx")
	if result != "" {
		t.Errorf("all lines should be filtered, got %q", result)
	}
}

func TestParseKeepassxcStderr_PreservesNonPromptColon(t *testing.T) {
	// A line ending with ":" but NOT containing the db name should be preserved
	result := parseKeepassxcStderr("Error: something went wrong:", "/test/db.kdbx")
	if result != "Error: something went wrong:" {
		t.Errorf("non-prompt colon line should be preserved, got %q", result)
	}
}

// -------------------------------------------------------------------
// Close — idempotency
// -------------------------------------------------------------------

func TestClose_Idempotent(t *testing.T) {
	c := &Client{
		masterPassword: []byte("secret"),
		passwordSet:    true,
	}

	c.Close()
	// Should not panic on double close
	c.Close()

	if c.passwordSet {
		t.Error("passwordSet should be false after double Close")
	}
}

// -------------------------------------------------------------------
// EnsureUnlocked — already unlocked
// -------------------------------------------------------------------

func TestEnsureUnlocked_AlreadyUnlocked(t *testing.T) {
	c := &Client{
		masterPassword: []byte("secret"),
		passwordSet:    true,
	}

	// Should return nil immediately without prompting
	err := c.EnsureUnlocked()
	if err != nil {
		t.Errorf("EnsureUnlocked should return nil when already unlocked, got: %v", err)
	}
}

