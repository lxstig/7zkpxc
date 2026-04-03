package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -------------------------------------------------------------------
// partitionBySize
// -------------------------------------------------------------------

func TestPartitionBySize_AllCategories(t *testing.T) {
	entries := []entryInfo{
		{EntryPath: "a/e1", StoredSize: 100}, // matches
		{EntryPath: "a/e2", StoredSize: 200}, // mismatch
		{EntryPath: "a/e3", StoredSize: 0},   // no metadata
		{EntryPath: "a/e4", StoredSize: 100}, // matches
	}

	candidates, noMeta, mismatch := partitionBySize(entries, 100)

	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
	if len(noMeta) != 1 {
		t.Errorf("expected 1 noMetadata, got %d", len(noMeta))
	}
	if len(mismatch) != 1 {
		t.Errorf("expected 1 mismatch, got %d", len(mismatch))
	}
}

func TestPartitionBySize_AllMatch(t *testing.T) {
	entries := []entryInfo{
		{EntryPath: "a/e1", StoredSize: 500},
		{EntryPath: "a/e2", StoredSize: 500},
	}

	candidates, noMeta, mismatch := partitionBySize(entries, 500)
	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(candidates))
	}
	if len(noMeta) != 0 {
		t.Errorf("expected 0 noMetadata, got %d", len(noMeta))
	}
	if len(mismatch) != 0 {
		t.Errorf("expected 0 mismatch, got %d", len(mismatch))
	}
}

func TestPartitionBySize_AllNoMetadata(t *testing.T) {
	entries := []entryInfo{
		{EntryPath: "a/e1", StoredSize: 0},
		{EntryPath: "a/e2", StoredSize: 0},
	}

	candidates, noMeta, mismatch := partitionBySize(entries, 100)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(noMeta) != 2 {
		t.Errorf("expected 2 noMetadata, got %d", len(noMeta))
	}
	if len(mismatch) != 0 {
		t.Errorf("expected 0 mismatch, got %d", len(mismatch))
	}
}

func TestPartitionBySize_Empty(t *testing.T) {
	candidates, noMeta, mismatch := partitionBySize(nil, 100)
	if len(candidates) != 0 || len(noMeta) != 0 || len(mismatch) != 0 {
		t.Error("expected all empty slices for nil input")
	}
}

func TestPartitionBySize_BothZero(t *testing.T) {
	entries := []entryInfo{
		{EntryPath: "a/e1", StoredSize: 0},
	}

	// When both archive size and stored size are 0, treat as no-metadata
	candidates, noMeta, mismatch := partitionBySize(entries, 0)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(candidates))
	}
	if len(noMeta) != 1 {
		t.Errorf("expected 1 noMetadata (both zero case), got %d", len(noMeta))
	}
	if len(mismatch) != 0 {
		t.Errorf("expected 0 mismatch, got %d", len(mismatch))
	}
}

// -------------------------------------------------------------------
// removeEntry
// -------------------------------------------------------------------

func TestRemoveEntry(t *testing.T) {
	entries := []entryInfo{
		{EntryPath: "a/e1"},
		{EntryPath: "a/e2"},
		{EntryPath: "a/e3"},
	}

	result := removeEntry(entries, "a/e2")
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}
	for _, e := range result {
		if e.EntryPath == "a/e2" {
			t.Error("removed entry should not be present")
		}
	}
}

func TestRemoveEntry_NotFound(t *testing.T) {
	entries := []entryInfo{
		{EntryPath: "a/e1"},
		{EntryPath: "a/e2"},
	}

	result := removeEntry(entries, "a/nonexistent")
	if len(result) != 2 {
		t.Errorf("expected 2 entries (nothing removed), got %d", len(result))
	}
}

func TestRemoveEntry_Empty(t *testing.T) {
	result := removeEntry(nil, "a/e1")
	if len(result) != 0 {
		t.Errorf("expected empty result, got %d", len(result))
	}
}

// -------------------------------------------------------------------
// findArchivesInDir
// -------------------------------------------------------------------

func TestFindArchivesInDir_Mixed(t *testing.T) {
	tmpDir := t.TempDir()

	// Create various files
	for _, name := range []string{
		"archive1.7z",
		"archive2.7z",
		"split.7z.001",
		"split.7z.002", // should NOT match — only .001 is first volume
		"document.pdf",
		"notes.txt",
		"photo.jpg",
	} {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create a subdirectory (should be skipped)
	if err := os.Mkdir(filepath.Join(tmpDir, "subdir.7z"), 0755); err != nil {
		t.Fatal(err)
	}

	archives, err := findArchivesInDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find: archive1.7z, archive2.7z, split.7z.001
	if len(archives) != 3 {
		t.Errorf("expected 3 archives, got %d: %v", len(archives), archives)
	}
}

func TestFindArchivesInDir_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	archives, err := findArchivesInDir(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(archives) != 0 {
		t.Errorf("expected 0 archives in empty dir, got %d", len(archives))
	}
}

func TestFindArchivesInDir_NonexistentDir(t *testing.T) {
	_, err := findArchivesInDir("/nonexistent/path/12345")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

// -------------------------------------------------------------------
// printRelinkSummary — stdout capture test
// -------------------------------------------------------------------

func TestPrintRelinkSummary_AllStatuses(t *testing.T) {
	// This tests that printRelinkSummary doesn't panic with all status types
	results := []relinkResult{
		{"archive1.7z", "relinked", "archive1.7z (deadbeef)"},
		{"archive2.7z", "verified", ""},
		{"archive3.7z", "no_match", ""},
		{"archive4.7z", "unencrypted", ""},
		{"archive5.7z", "error", "some error"},
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printRelinkSummary(results)

	_ = w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	// Verify all categories appear
	if !strings.Contains(output, "Relinked") {
		t.Error("output should contain 'Relinked'")
	}
	if !strings.Contains(output, "Verified") {
		t.Error("output should contain 'Verified'")
	}
	if !strings.Contains(output, "No match") {
		t.Error("output should contain 'No match'")
	}
	if !strings.Contains(output, "Unencrypted") {
		t.Error("output should contain 'Unencrypted'")
	}
	if !strings.Contains(output, "Errors") {
		t.Error("output should contain 'Errors'")
	}
}

func TestPrintRelinkSummary_Empty(t *testing.T) {
	// Should not panic with empty results
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printRelinkSummary(nil)

	_ = w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Relink Summary") {
		t.Error("output should contain 'Relink Summary' header")
	}
}

func TestPrintRelinkSummary_OnlyRelinked(t *testing.T) {
	results := []relinkResult{
		{"a.7z", "relinked", "a.7z (abc12345)"},
		{"b.7z", "relinked", "b.7z (def67890)"},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printRelinkSummary(results)

	_ = w.Close()
	os.Stdout = old

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	if !strings.Contains(output, "Relinked") {
		t.Error("output should contain 'Relinked'")
	}
	// Should NOT show other categories
	if strings.Contains(output, "No match") {
		t.Error("output should not contain 'No match' when there are none")
	}
}
