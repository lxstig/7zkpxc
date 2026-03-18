package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFile_SameDevice(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "source.7z")
	dst := filepath.Join(tmpDir, "dest.7z")

	if err := os.WriteFile(src, []byte("archive data"), 0644); err != nil {
		t.Fatal(err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	crossDevice, err := moveFile(src, dst, srcInfo)
	if err != nil {
		t.Fatalf("moveFile failed: %v", err)
	}

	if crossDevice {
		t.Error("moveFile reported cross-device for same-dir move")
	}

	// Source should be gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source file still exists after move")
	}

	// Destination should exist with correct content
	content, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(content) != "archive data" {
		t.Errorf("destination content = %q, want %q", content, "archive data")
	}
}

func TestMoveFileCopy(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "source.7z")
	dst := filepath.Join(tmpDir, "subdir", "dest.7z")

	content := []byte("test archive content with some data")
	if err := os.WriteFile(src, content, 0644); err != nil {
		t.Fatal(err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}

	if err := moveFileCopy(src, dst, srcInfo); err != nil {
		t.Fatalf("moveFileCopy failed: %v", err)
	}

	// Source should be gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source file still exists after copy-move")
	}

	// Destination should have correct content
	dstContent, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("destination content mismatch")
	}
}

func TestMoveFileCopy_DestExists(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "source.7z")
	dst := filepath.Join(tmpDir, "dest.7z")

	if err := os.WriteFile(src, []byte("src"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("dst"), 0644); err != nil {
		t.Fatal(err)
	}

	srcInfo, _ := os.Stat(src)

	// moveFileCopy uses O_EXCL, so it should fail if dst exists
	err := moveFileCopy(src, dst, srcInfo)
	if err == nil {
		t.Error("moveFileCopy should fail when destination exists")
	}
}

func TestRollbackMove_SameDevice(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "original.7z")
	dst := filepath.Join(tmpDir, "moved.7z")

	if err := os.WriteFile(dst, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	srcInfo, _ := os.Stat(dst)

	// Rollback: move dst back to src (same device)
	if err := rollbackMove(src, dst, srcInfo, false); err != nil {
		t.Fatalf("rollbackMove failed: %v", err)
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Error("source should exist after rollback")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("destination should not exist after rollback")
	}
}
