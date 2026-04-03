package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// -------------------------------------------------------------------
// getCompressionFlags
// -------------------------------------------------------------------

func TestGetCompressionFlags_Default(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")

	flags := getCompressionFlags(cmd)
	if len(flags) != 0 {
		t.Errorf("expected no flags for default, got %v", flags)
	}
}

func TestGetCompressionFlags_Fast(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")
	_ = cmd.Flags().Set("fast", "true")

	flags := getCompressionFlags(cmd)
	if len(flags) != 1 || flags[0] != "-mx=1" {
		t.Errorf("expected [-mx=1], got %v", flags)
	}
}

func TestGetCompressionFlags_Best(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")
	_ = cmd.Flags().Set("best", "true")

	flags := getCompressionFlags(cmd)
	if len(flags) != 1 || flags[0] != "-mx=9" {
		t.Errorf("expected [-mx=9], got %v", flags)
	}
}

func TestGetCompressionFlags_FastTakesPrecedence(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")
	_ = cmd.Flags().Set("fast", "true")
	_ = cmd.Flags().Set("best", "true")

	flags := getCompressionFlags(cmd)
	// fast is checked first
	if len(flags) != 1 || flags[0] != "-mx=1" {
		t.Errorf("expected [-mx=1] (fast takes precedence), got %v", flags)
	}
}

// -------------------------------------------------------------------
// buildCompressionArgs
// -------------------------------------------------------------------

func TestBuildCompressionArgs_Default(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")
	cmd.Flags().String("volume", "", "")

	args := buildCompressionArgs(cmd, []string{"archive.7z", "file.txt", "-p", "-mhe=on"})
	// Should start with "a"
	if len(args) < 1 || args[0] != "a" {
		t.Errorf("first arg should be 'a', got %v", args)
	}
	// Should contain the default args
	found := false
	for _, a := range args {
		if a == "-mhe=on" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -mhe=on in args: %v", args)
	}
}

func TestBuildCompressionArgs_WithVolume(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")
	cmd.Flags().String("volume", "", "")
	_ = cmd.Flags().Set("volume", "100m")

	args := buildCompressionArgs(cmd, []string{"archive.7z"})
	found := false
	for _, a := range args {
		if a == "-v100m" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -v100m in args: %v", args)
	}
}

func TestBuildCompressionArgs_WithFastAndVolume(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("fast", false, "")
	cmd.Flags().Bool("best", false, "")
	cmd.Flags().String("volume", "", "")
	_ = cmd.Flags().Set("fast", "true")
	_ = cmd.Flags().Set("volume", "50m")

	args := buildCompressionArgs(cmd, []string{"archive.7z"})
	hasFast := false
	hasVolume := false
	for _, a := range args {
		if a == "-mx=1" {
			hasFast = true
		}
		if a == "-v50m" {
			hasVolume = true
		}
	}
	if !hasFast {
		t.Errorf("expected -mx=1 in args: %v", args)
	}
	if !hasVolume {
		t.Errorf("expected -v50m in args: %v", args)
	}
}

// -------------------------------------------------------------------
// updateMetadata
// -------------------------------------------------------------------

func TestUpdateMetadata_NewFile(t *testing.T) {
	mock := NewMockPasswordProvider()
	entryPath := "backups/archive.7z (deadbeef)"

	// Create a real file so os.Stat works
	tmpDir := t.TempDir()
	archiveFile := filepath.Join(tmpDir, "archive.7z")
	if err := os.WriteFile(archiveFile, []byte("test archive content"), 0644); err != nil {
		t.Fatal(err)
	}

	// No existing Notes
	updateMetadata(mock, entryPath, archiveFile)

	// Check that Notes were updated
	notes, err := mock.GetAttribute(entryPath, "Notes")
	if err != nil {
		t.Fatalf("Notes should be set after updateMetadata: %v", err)
	}

	// Should contain [7zkpxc] section with size
	m := parseMetadata(notes)
	info, _ := os.Stat(archiveFile)
	if m.Size != info.Size() {
		t.Errorf("Size = %d, want %d", m.Size, info.Size())
	}
}

func TestUpdateMetadata_SkipsIfCurrent(t *testing.T) {
	mock := NewMockPasswordProvider()
	entryPath := "backups/archive.7z (deadbeef)"

	tmpDir := t.TempDir()
	archiveFile := filepath.Join(tmpDir, "archive.7z")
	content := []byte("test archive content")
	if err := os.WriteFile(archiveFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(archiveFile)
	existingNotes := buildMetadataSection(EntryMetadata{Size: info.Size(), Ver: "2.8.3"})
	mock.SetAttribute(entryPath, "Notes", existingNotes)

	updateMetadata(mock, entryPath, archiveFile)

	// Should NOT call UpdateEntryNotes (skip because already current)
	for _, c := range mock.GetCalls() {
		if c == "update-notes:"+entryPath {
			t.Error("updateMetadata should skip when metadata is current")
		}
	}
}

func TestUpdateMetadata_NonexistentFile(t *testing.T) {
	mock := NewMockPasswordProvider()
	// Should not panic — just return silently
	updateMetadata(mock, "entry", "/nonexistent/file.7z")
}
