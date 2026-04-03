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

func TestRollbackMove_CrossDevice(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(tmpDir, "original.7z")
	dst := filepath.Join(subDir, "moved.7z")

	content := []byte("cross device data")
	if err := os.WriteFile(dst, content, 0644); err != nil {
		t.Fatal(err)
	}

	dstInfo, _ := os.Stat(dst)

	// crossDevice=true → uses moveFileCopy internally
	if err := rollbackMove(src, dst, dstInfo, true); err != nil {
		t.Fatalf("rollbackMove (cross-device) failed: %v", err)
	}

	// src should now exist with correct content
	if _, err := os.Stat(src); os.IsNotExist(err) {
		t.Error("source should exist after cross-device rollback")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("failed to read rolled-back file: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("content mismatch after rollback: got %q", data)
	}
	// dst should be removed (moveFileCopy deletes source after copy)
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("destination should not exist after cross-device rollback")
	}
}

func TestMoveFile_DestDirNotExist(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "source.7z")
	dst := filepath.Join(tmpDir, "nonexistent", "deep", "dest.7z")

	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	srcInfo, _ := os.Stat(src)

	// Rename should fail because destination directory doesn't exist
	_, err := moveFile(src, dst, srcInfo)
	if err == nil {
		t.Error("moveFile should fail when destination directory doesn't exist")
	}
}

func TestMoveFileCopy_SourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "nonexistent.7z")
	dst := filepath.Join(tmpDir, "dest.7z")

	// Use a dummy FileInfo from a real file
	dummyFile := filepath.Join(tmpDir, "dummy")
	if err := os.WriteFile(dummyFile, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	dummyInfo, _ := os.Stat(dummyFile)

	err := moveFileCopy(src, dst, dummyInfo)
	if err == nil {
		t.Error("moveFileCopy should fail when source doesn't exist")
	}
}

func TestMoveFileCopy_PreservesPermissions(t *testing.T) {
	tmpDir := t.TempDir()

	src := filepath.Join(tmpDir, "source.7z")
	dst := filepath.Join(tmpDir, "dest.7z")

	content := []byte("sensitive archive data")
	if err := os.WriteFile(src, content, 0600); err != nil {
		t.Fatal(err)
	}

	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}

	if err := moveFileCopy(src, dst, srcInfo); err != nil {
		t.Fatalf("moveFileCopy failed: %v", err)
	}

	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("destination doesn't exist: %v", err)
	}

	if dstInfo.Mode() != srcInfo.Mode() {
		t.Errorf("permissions not preserved: got %v, want %v", dstInfo.Mode(), srcInfo.Mode())
	}
}
