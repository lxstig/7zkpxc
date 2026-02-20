package app

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// Regex patterns for detecting split volumes
var (
	// Standard split format: archive.ext.001, archive.ext.002, etc.
	// Supports: 7z, zip, tar, gz, xz, bz2, rar
	splitStandardRe = regexp.MustCompile(`(?i)\.(7z|zip|tar|gz|xz|bz2|rar)\.[0-9]{3,}$`)

	// RAR part format: archive.part001.rar, archive.part002.rar, etc.
	splitRarPartRe = regexp.MustCompile(`(?i)\.part[0-9]{3,}\.rar$`)

	// RAR old format: archive.rar, archive.r00, archive.r01, etc.
	splitRarOldRe = regexp.MustCompile(`(?i)\.r[0-9]{2,}$`)
)

// ArchiveType represents the type of archive detected
type ArchiveType int

const (
	ArchiveStandard ArchiveType = iota
	ArchiveSplitStandard
	ArchiveSplitRarPart
	ArchiveSplitRarOld
)

// ArchiveInfo contains metadata about an archive file
type ArchiveInfo struct {
	OriginalName   string
	NormalizedName string
	Type           ArchiveType
	IsSplit        bool
}

// AnalyzeArchive analyzes an archive path and returns metadata
func AnalyzeArchive(path string) ArchiveInfo {
	base := filepath.Base(path)
	normalized := normalizeArchiveName(path)

	info := ArchiveInfo{
		OriginalName:   base,
		NormalizedName: normalized,
		Type:           ArchiveStandard,
		IsSplit:        normalized != base,
	}

	switch {
	case splitStandardRe.MatchString(base):
		info.Type = ArchiveSplitStandard
	case splitRarPartRe.MatchString(base):
		info.Type = ArchiveSplitRarPart
	case splitRarOldRe.MatchString(base):
		info.Type = ArchiveSplitRarOld
	}

	return info
}

// normalizeArchiveName extracts the logical archive name from a file path.
// It handles standard archives and various split volume formats:
//   - Standard split: archive.7z.001 -> archive.7z
//   - RAR part: archive.part001.rar -> archive.rar
//   - RAR old: archive.r00 -> archive.rar
func normalizeArchiveName(path string) string {
	// Normalize path separators for cross-platform base name extraction
	base := filepath.Base(strings.ReplaceAll(path, "\\", "/"))

	// Standard split format: *.ext.NNN
	if splitStandardRe.MatchString(base) {
		ext := filepath.Ext(base) // .001
		return strings.TrimSuffix(base, ext)
	}

	// RAR part format: *.partNNN.rar -> *.rar (preserve original case)
	if splitRarPartRe.MatchString(base) {
		loc := splitRarPartRe.FindStringIndex(base)
		if loc != nil {
			matched := base[loc[0]:loc[1]]
			rarExt := matched[len(matched)-4:] // ".rar", ".RAR", ".Rar", etc.
			return base[:loc[0]] + rarExt
		}
	}

	// RAR old format: *.rNN -> *.rar (preserve case from original extension)
	if splitRarOldRe.MatchString(base) {
		ext := filepath.Ext(base) // .r00 or .R00
		prefix := strings.TrimSuffix(base, ext)
		if ext[1] >= 'A' && ext[1] <= 'Z' {
			return prefix + ".RAR"
		}
		return prefix + ".rar"
	}

	return base
}

// joinEntry joins a prefix path with an entry name, handling slash separators.
// It ensures consistent forward-slash separators regardless of OS.
func joinEntry(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return strings.ReplaceAll(filepath.Join(prefix, name), "\\", "/")
}

// GetPasswordForArchive attempts to find the password for an archive.
// It implements a fallback strategy:
//  1. Try normalized name (e.g., archive.7z.001 -> archive.7z)
//  2. Try original filename (handles edge cases like backup.2024)
//  3. For split archives, try additional variants
//
// Returns the password if found, or an error with details about what was tried.
func GetPasswordForArchive(kp PasswordProvider, entryPathPrefix, archivePath string) ([]byte, error) {
	if kp == nil {
		return nil, fmt.Errorf("password provider is nil")
	}

	if archivePath == "" {
		return nil, fmt.Errorf("archive path is empty")
	}

	info := AnalyzeArchive(archivePath)
	tried := make([]string, 0, 3)

	// 1. Try normalized name (primary strategy for split volumes)
	normalizedKey := joinEntry(entryPathPrefix, info.NormalizedName)
	tried = append(tried, normalizedKey)
	if pass, err := kp.GetPassword(normalizedKey); err == nil {
		return pass, nil
	}

	// 2. Try original base name (fallback for non-standard naming)
	if info.NormalizedName != info.OriginalName {
		originalKey := joinEntry(entryPathPrefix, info.OriginalName)
		tried = append(tried, originalKey)
		if pass, err := kp.GetPassword(originalKey); err == nil {
			return pass, nil
		}
	}

	// 3. For split archives, try base name without extension
	//    (e.g., archive.7z.001 -> try "archive" as last resort)
	if info.IsSplit {
		baseWithoutExt := strings.TrimSuffix(info.NormalizedName, filepath.Ext(info.NormalizedName))
		if baseWithoutExt != info.NormalizedName && baseWithoutExt != info.OriginalName {
			baseKey := joinEntry(entryPathPrefix, baseWithoutExt)
			tried = append(tried, baseKey)
			if pass, err := kp.GetPassword(baseKey); err == nil {
				return pass, nil
			}
		}
	}

	return nil, &PasswordNotFoundError{
		ArchiveName: info.OriginalName,
		Tried:       tried,
	}
}

// PasswordProvider is an interface for password retrieval
type PasswordProvider interface {
	GetPassword(key string) ([]byte, error)
}

// PasswordNotFoundError is returned when no password can be found for an archive
type PasswordNotFoundError struct {
	ArchiveName string
	Tried       []string
}

func (e *PasswordNotFoundError) Error() string {
	return fmt.Sprintf("password not found for archive '%s' (tried: %s)",
		e.ArchiveName, strings.Join(e.Tried, ", "))
}

// IsPasswordNotFound checks if an error is a PasswordNotFoundError
func IsPasswordNotFound(err error) bool {
	_, ok := err.(*PasswordNotFoundError)
	return ok
}
