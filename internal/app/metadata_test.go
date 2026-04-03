package app

import (
	"strings"
	"testing"
)

// -------------------------------------------------------------------
// parseMetadata
// -------------------------------------------------------------------

func TestParseMetadata_Empty(t *testing.T) {
	m := parseMetadata("")
	if m.Size != 0 {
		t.Errorf("Size = %d, want 0", m.Size)
	}
	if m.Ver != "" {
		t.Errorf("Ver = %q, want empty", m.Ver)
	}
}

func TestParseMetadata_ValidSection(t *testing.T) {
	notes := "[7zkpxc]\nsize=12345\nver=2.8.3\n"
	m := parseMetadata(notes)
	if m.Size != 12345 {
		t.Errorf("Size = %d, want 12345", m.Size)
	}
	if m.Ver != "2.8.3" {
		t.Errorf("Ver = %q, want %q", m.Ver, "2.8.3")
	}
}

func TestParseMetadata_WithUserNotes(t *testing.T) {
	notes := "My personal notes\nSome info here\n\n[7zkpxc]\nsize=999\nver=1.0.0\n\n[other]\nkey=val"
	m := parseMetadata(notes)
	if m.Size != 999 {
		t.Errorf("Size = %d, want 999", m.Size)
	}
	if m.Ver != "1.0.0" {
		t.Errorf("Ver = %q, want %q", m.Ver, "1.0.0")
	}
}

func TestParseMetadata_NoSection(t *testing.T) {
	notes := "Just some user notes\nwithout any metadata section"
	m := parseMetadata(notes)
	if m.Size != 0 || m.Ver != "" {
		t.Errorf("expected zero metadata, got Size=%d, Ver=%q", m.Size, m.Ver)
	}
}

func TestParseMetadata_PartialFields(t *testing.T) {
	notes := "[7zkpxc]\nsize=42\n"
	m := parseMetadata(notes)
	if m.Size != 42 {
		t.Errorf("Size = %d, want 42", m.Size)
	}
	if m.Ver != "" {
		t.Errorf("Ver = %q, want empty", m.Ver)
	}
}

func TestParseMetadata_InvalidSize(t *testing.T) {
	notes := "[7zkpxc]\nsize=notanumber\nver=2.0.0\n"
	m := parseMetadata(notes)
	if m.Size != 0 {
		t.Errorf("Size = %d, want 0 for invalid value", m.Size)
	}
	if m.Ver != "2.0.0" {
		t.Errorf("Ver = %q, want %q", m.Ver, "2.0.0")
	}
}

func TestParseMetadata_MalformedLines(t *testing.T) {
	notes := "[7zkpxc]\nno_equals_sign\nsize=100\n"
	m := parseMetadata(notes)
	if m.Size != 100 {
		t.Errorf("Size = %d, want 100", m.Size)
	}
}

// -------------------------------------------------------------------
// buildMetadataSection
// -------------------------------------------------------------------

func TestBuildMetadataSection_Full(t *testing.T) {
	m := EntryMetadata{Size: 12345, Ver: "2.8.3"}
	result := buildMetadataSection(m)
	if !strings.Contains(result, "[7zkpxc]") {
		t.Error("missing [7zkpxc] header")
	}
	if !strings.Contains(result, "size=12345") {
		t.Error("missing size=12345")
	}
	if !strings.Contains(result, "ver=2.8.3") {
		t.Error("missing ver=2.8.3")
	}
}

func TestBuildMetadataSection_Empty(t *testing.T) {
	m := EntryMetadata{}
	result := buildMetadataSection(m)
	if !strings.Contains(result, "[7zkpxc]") {
		t.Error("missing [7zkpxc] header")
	}
	if strings.Contains(result, "size=") {
		t.Error("should not contain size= when Size is 0")
	}
	if strings.Contains(result, "ver=") {
		t.Error("should not contain ver= when Ver is empty")
	}
}

func TestBuildMetadataSection_OnlySize(t *testing.T) {
	m := EntryMetadata{Size: 999}
	result := buildMetadataSection(m)
	if !strings.Contains(result, "size=999") {
		t.Error("missing size=999")
	}
	if strings.Contains(result, "ver=") {
		t.Error("should not contain ver= when Ver is empty")
	}
}

// -------------------------------------------------------------------
// mergeMetadataIntoNotes
// -------------------------------------------------------------------

func TestMergeMetadataIntoNotes_Append(t *testing.T) {
	existing := "User notes here"
	m := EntryMetadata{Size: 100, Ver: "2.0.0"}
	result := mergeMetadataIntoNotes(existing, m)
	if !strings.Contains(result, "User notes here") {
		t.Error("user notes should be preserved")
	}
	if !strings.Contains(result, "[7zkpxc]") {
		t.Error("should contain [7zkpxc] section")
	}
	if !strings.Contains(result, "size=100") {
		t.Error("should contain size=100")
	}
}

func TestMergeMetadataIntoNotes_Replace(t *testing.T) {
	existing := "[7zkpxc]\nsize=50\nver=1.0.0\n"
	m := EntryMetadata{Size: 200, Ver: "2.0.0"}
	result := mergeMetadataIntoNotes(existing, m)
	if strings.Contains(result, "size=50") {
		t.Error("old size should be replaced")
	}
	if !strings.Contains(result, "size=200") {
		t.Error("should contain new size=200")
	}
	if strings.Contains(result, "ver=1.0.0") {
		t.Error("old version should be replaced")
	}
	if !strings.Contains(result, "ver=2.0.0") {
		t.Error("should contain new ver=2.0.0")
	}
}

func TestMergeMetadataIntoNotes_EmptyExisting(t *testing.T) {
	m := EntryMetadata{Size: 100, Ver: "2.0.0"}
	result := mergeMetadataIntoNotes("", m)
	if !strings.HasPrefix(result, "[7zkpxc]") {
		t.Error("should start with [7zkpxc] when existing is empty")
	}
}

func TestMergeMetadataIntoNotes_PreserveOtherSections(t *testing.T) {
	existing := "Notes\n[7zkpxc]\nsize=1\nver=1.0\n\n[custom]\nkey=val\n"
	m := EntryMetadata{Size: 2, Ver: "2.0"}
	result := mergeMetadataIntoNotes(existing, m)
	if !strings.Contains(result, "[custom]") {
		t.Error("other sections should be preserved")
	}
	if !strings.Contains(result, "key=val") {
		t.Error("other section content should be preserved")
	}
}

// -------------------------------------------------------------------
// Roundtrip
// -------------------------------------------------------------------

func TestMetadata_Roundtrip(t *testing.T) {
	original := EntryMetadata{Size: 54321, Ver: "2.8.3"}
	section := buildMetadataSection(original)
	parsed := parseMetadata(section)

	if parsed.Size != original.Size {
		t.Errorf("Size roundtrip: %d → %d", original.Size, parsed.Size)
	}
	if parsed.Ver != original.Ver {
		t.Errorf("Ver roundtrip: %q → %q", original.Ver, parsed.Ver)
	}
}
