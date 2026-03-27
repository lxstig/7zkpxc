package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const metadataHeader = "[7zkpxc]"

// EntryMetadata holds structured metadata stored in a KeePass entry's Notes field.
type EntryMetadata struct {
	Size int64  // archive file size in bytes; 0 means unknown
	Ver  string // 7zkpxc version that last updated this entry
}

// parseMetadata extracts EntryMetadata from a Notes string.
// Returns zero-value fields for missing keys.
func parseMetadata(notes string) EntryMetadata {
	var m EntryMetadata
	inSection := false

	for _, line := range strings.Split(notes, "\n") {
		line = strings.TrimSpace(line)

		if line == metadataHeader {
			inSection = true
			continue
		}

		// A new section header or blank line after our section ends parsing
		if inSection && (strings.HasPrefix(line, "[") || line == "") {
			break
		}

		if !inSection {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "size":
			m.Size, _ = strconv.ParseInt(val, 10, 64)
		case "ver":
			m.Ver = val
		}
	}

	return m
}

// buildMetadataSection returns the [7zkpxc] INI section string.
func buildMetadataSection(m EntryMetadata) string {
	var b strings.Builder
	b.WriteString(metadataHeader)
	b.WriteByte('\n')
	if m.Size > 0 {
		fmt.Fprintf(&b, "size=%d\n", m.Size)
	}
	if m.Ver != "" {
		fmt.Fprintf(&b, "ver=%s\n", m.Ver)
	}
	return b.String()
}

// mergeMetadataIntoNotes replaces (or appends) the [7zkpxc] section in notes.
// User content outside the section is preserved.
func mergeMetadataIntoNotes(existingNotes string, m EntryMetadata) string {
	newSection := buildMetadataSection(m)

	if !strings.Contains(existingNotes, metadataHeader) {
		// No existing section — append
		if existingNotes != "" && !strings.HasSuffix(existingNotes, "\n") {
			existingNotes += "\n"
		}
		return existingNotes + newSection
	}

	// Replace existing section
	var result strings.Builder
	inSection := false
	for _, line := range strings.Split(existingNotes, "\n") {
		trimmed := strings.TrimSpace(line)

		if trimmed == metadataHeader {
			inSection = true
			result.WriteString(newSection)
			continue
		}

		if inSection {
			// Skip old section lines until next section or blank line
			if strings.HasPrefix(trimmed, "[") || trimmed == "" {
				inSection = false
				result.WriteString(line)
				result.WriteByte('\n')
			}
			continue
		}

		result.WriteString(line)
		result.WriteByte('\n')
	}

	return strings.TrimRight(result.String(), "\n")
}

// updateMetadata refreshes the [7zkpxc] metadata in a KeePass entry's Notes.
// This is a non-fatal housekeeping operation — errors are silently ignored.
func updateMetadata(kp PasswordProvider, entryPath, absArchivePath string) {
	info, err := os.Stat(absArchivePath)
	if err != nil {
		return
	}

	currentNotes, _ := kp.GetAttribute(entryPath, "Notes")
	meta := parseMetadata(currentNotes)

	// Skip if nothing changed
	if meta.Size == info.Size() && meta.Ver != "" {
		return
	}

	meta.Size = info.Size()
	meta.Ver = appVersion
	newNotes := mergeMetadataIntoNotes(currentNotes, meta)
	_ = kp.UpdateEntryNotes(entryPath, newNotes) // non-fatal
}
