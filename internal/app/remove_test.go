package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "test.7z")

	if err := os.WriteFile(f, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	removeSingleFile(f)

	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Error("file should be deleted after removeSingleFile")
	}
}

func TestRemoveSingleFile_NonExistent(t *testing.T) {
	// Should not panic on non-existent file
	removeSingleFile("/nonexistent/path/file.7z")
}

func TestRemoveAllSplitVolumes(t *testing.T) {
	tmpDir := t.TempDir()

	// Create fake split volumes
	for _, name := range []string{"archive.7z.001", "archive.7z.002", "archive.7z.003"} {
		f := filepath.Join(tmpDir, name)
		if err := os.WriteFile(f, []byte("vol"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Also create an unrelated file that should NOT be deleted
	unrelated := filepath.Join(tmpDir, "other.txt")
	if err := os.WriteFile(unrelated, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	removeAllSplitVolumes(filepath.Join(tmpDir, "archive.7z.001"))

	// All split volumes should be gone
	for _, name := range []string{"archive.7z.001", "archive.7z.002", "archive.7z.003"} {
		f := filepath.Join(tmpDir, name)
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("split volume %q should have been deleted", name)
		}
	}

	// Unrelated file should still exist
	if _, err := os.Stat(unrelated); os.IsNotExist(err) {
		t.Error("unrelated file should not have been deleted")
	}
}

func TestRemoveAllSplitVolumes_RarPart(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"backup.part001.rar", "backup.part002.rar"} {
		f := filepath.Join(tmpDir, name)
		if err := os.WriteFile(f, []byte("vol"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	removeAllSplitVolumes(filepath.Join(tmpDir, "backup.part001.rar"))

	for _, name := range []string{"backup.part001.rar", "backup.part002.rar"} {
		f := filepath.Join(tmpDir, name)
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("RAR part %q should have been deleted", name)
		}
	}
}

func TestRemoveAllSplitVolumes_RarOld(t *testing.T) {
	tmpDir := t.TempDir()

	for _, name := range []string{"data.r00", "data.r01", "data.r02"} {
		f := filepath.Join(tmpDir, name)
		if err := os.WriteFile(f, []byte("vol"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	removeAllSplitVolumes(filepath.Join(tmpDir, "data.r00"))

	for _, name := range []string{"data.r00", "data.r01", "data.r02"} {
		f := filepath.Join(tmpDir, name)
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("RAR old volume %q should have been deleted", name)
		}
	}
}
