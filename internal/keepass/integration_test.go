package keepass

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const testMasterPassword = "TestMasterP@ss123!"

// createTestDB creates a fresh KDBX database with a known password.
func createTestDB(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed")
	}

	dbPath := filepath.Join(t.TempDir(), "test.kdbx")
	cmd := exec.Command("keepassxc-cli", "db-create", "-p", dbPath)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANGUAGE=en_US.UTF-8", "LANG=en_US.UTF-8")
	cmd.Stdin = strings.NewReader(testMasterPassword + "\n" + testMasterPassword + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("db-create failed: %v\n%s", err, out)
	}
	return dbPath
}

func newTestClient(t *testing.T, dbPath string) *Client {
	t.Helper()
	c := New(dbPath)
	c.masterPassword = []byte(testMasterPassword)
	c.passwordSet = true
	t.Cleanup(func() { c.Close() })
	return c
}

func TestIntegration_VerifyConnection(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	if err := c.VerifyConnection(); err != nil {
		t.Fatalf("VerifyConnection: %v", err)
	}
}

func TestIntegration_Mkdir_And_GroupExists(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	if err := c.Mkdir("Archives/Test"); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if !c.GroupExists("Archives/Test") {
		t.Error("group should exist")
	}
	if c.GroupExists("NonExistent/Group") {
		t.Error("non-existent group should return false")
	}
}

func TestIntegration_Mkdir_Idempotent(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	if err := c.Mkdir("Archives"); err != nil {
		t.Fatal(err)
	}
	if err := c.Mkdir("Archives"); err != nil {
		t.Fatalf("idempotent Mkdir should not fail: %v", err)
	}
}

func TestIntegration_AddEntry_GetPassword(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	pw := []byte("archive_secret_42!")
	if err := c.AddEntry("Grp", "myarchive.7z (deadbeef)", pw, "/home/user/a.7z", "https://github.com/lxstig/7zkpxc"); err != nil {
		t.Fatalf("AddEntry: %v", err)
	}
	got, err := c.GetPassword("Grp/myarchive.7z (deadbeef)")
	if err != nil {
		t.Fatalf("GetPassword: %v", err)
	}
	if string(got) != string(pw) {
		t.Errorf("password = %q, want %q", got, pw)
	}
}

func TestIntegration_GetPassword_NotFound(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	if _, err := c.GetPassword("NonExistent/entry"); err == nil {
		t.Fatal("expected error for missing entry")
	}
}

func TestIntegration_GetAttribute(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	_ = c.AddEntry("G", "e (aabb1122)", []byte("pw"), "/path/to/f.7z", "https://example.com")

	if u, err := c.GetAttribute("G/e (aabb1122)", "Username"); err != nil {
		t.Fatalf("GetAttribute(Username): %v", err)
	} else if u != "/path/to/f.7z" {
		t.Errorf("Username = %q", u)
	}

	if u, err := c.GetAttribute("G/e (aabb1122)", "URL"); err != nil {
		t.Fatalf("GetAttribute(URL): %v", err)
	} else if u != "https://example.com" {
		t.Errorf("URL = %q", u)
	}
}

func TestIntegration_UpdateEntryUsername(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	_ = c.AddEntry("G", "f.7z (11223344)", []byte("pw"), "/old.7z", "")
	if err := c.UpdateEntryUsername("G/f.7z (11223344)", "/new.7z"); err != nil {
		t.Fatalf("UpdateEntryUsername: %v", err)
	}
	got, _ := c.GetAttribute("G/f.7z (11223344)", "Username")
	if got != "/new.7z" {
		t.Errorf("Username = %q, want /new.7z", got)
	}
}

func TestIntegration_UpdateEntryNotes(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	_ = c.AddEntry("G", "f.7z (aabbccdd)", []byte("pw"), "/p", "")

	notes := "[7zkpxc]\nsize: 1234\nversion: 1"
	if err := c.UpdateEntryNotes("G/f.7z (aabbccdd)", notes); err != nil {
		t.Fatalf("UpdateEntryNotes: %v", err)
	}
	got, _ := c.GetAttribute("G/f.7z (aabbccdd)", "Notes")
	if !strings.Contains(got, "size: 1234") {
		t.Errorf("Notes = %q", got)
	}
}

func TestIntegration_Search(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	_ = c.AddEntry("A", "backup.7z (11111111)", []byte("p1"), "/a", "")
	_ = c.AddEntry("A", "backup.7z (22222222)", []byte("p2"), "/b", "")
	_ = c.AddEntry("A", "other.7z (33333333)", []byte("p3"), "/c", "")

	results, err := c.Search("backup.7z")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("expected ≥2 results, got %d: %v", len(results), results)
	}

	none, err := c.Search("zzz_nonexistent_zzz")
	if err != nil {
		t.Fatalf("Search(nonexistent): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("expected 0 results, got %d", len(none))
	}
}

func TestIntegration_ListEntries(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	_ = c.AddEntry("MyGrp", "a.7z (aaaa1111)", []byte("p"), "/a", "")
	_ = c.AddEntry("MyGrp", "b.7z (bbbb2222)", []byte("p"), "/b", "")

	entries, err := c.ListEntries("MyGrp")
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2, got %d: %v", len(entries), entries)
	}
}

func TestIntegration_DeleteEntry(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	_ = c.AddEntry("G", "del.7z (deadbeef)", []byte("pw"), "/p", "")

	if _, err := c.GetPassword("G/del.7z (deadbeef)"); err != nil {
		t.Fatalf("should exist before delete: %v", err)
	}
	if err := c.DeleteEntry("G/del.7z (deadbeef)"); err != nil {
		t.Fatalf("DeleteEntry: %v", err)
	}
	if _, err := c.GetPassword("G/del.7z (deadbeef)"); err == nil {
		t.Error("should be gone after delete")
	}
}

func TestIntegration_EditEntryTitle(t *testing.T) {
	c := newTestClient(t, createTestDB(t))
	_ = c.AddEntry("G", "old.7z (11111111)", []byte("pw"), "/old.7z", "")

	if err := c.EditEntryTitle("G/old.7z (11111111)", "new.7z (11111111)", "/new.7z"); err != nil {
		t.Fatalf("EditEntryTitle: %v", err)
	}

	// Old gone
	if _, err := c.GetPassword("G/old.7z (11111111)"); err == nil {
		t.Error("old entry should be gone")
	}
	// New exists
	got, err := c.GetPassword("G/new.7z (11111111)")
	if err != nil {
		t.Fatalf("new entry missing: %v", err)
	}
	if string(got) != "pw" {
		t.Errorf("password = %q", got)
	}
	u, _ := c.GetAttribute("G/new.7z (11111111)", "Username")
	if u != "/new.7z" {
		t.Errorf("Username = %q", u)
	}
}

func TestIntegration_GeneratePassword(t *testing.T) {
	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		t.Skip("keepassxc-cli not installed")
	}
	c := &Client{}
	pw, err := c.GeneratePassword(64)
	if err != nil {
		t.Fatalf("GeneratePassword: %v", err)
	}
	if len(pw) < 60 {
		t.Errorf("too short: %d", len(pw))
	}
}
