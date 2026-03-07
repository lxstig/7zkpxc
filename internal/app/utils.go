package app

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
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

	// UUID entry title format: "basename (abcd1234)"
	uuidTitleRe = regexp.MustCompile(`^(.+) \(([0-9a-f]{8})\)$`)
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
func normalizeArchiveName(path string) string {
	base := filepath.Base(strings.ReplaceAll(path, "\\", "/"))

	if splitStandardRe.MatchString(base) {
		ext := filepath.Ext(base) // .001
		return strings.TrimSuffix(base, ext)
	}

	if splitRarPartRe.MatchString(base) {
		loc := splitRarPartRe.FindStringIndex(base)
		if loc != nil {
			matched := base[loc[0]:loc[1]]
			rarExt := matched[len(matched)-4:] // ".rar", ".RAR", etc.
			return base[:loc[0]] + rarExt
		}
	}

	if splitRarOldRe.MatchString(base) {
		ext := filepath.Ext(base)
		prefix := strings.TrimSuffix(base, ext)
		if ext[1] >= 'A' && ext[1] <= 'Z' {
			return prefix + ".RAR"
		}
		return prefix + ".rar"
	}

	return base
}

// joinEntry joins a prefix path with an entry name using forward slashes.
func joinEntry(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return strings.ReplaceAll(filepath.Join(prefix, name), "\\", "/")
}

// -------------------------------------------------------------------
// UUID-based entry title helpers
//
// New format: "basename (uuid8)"
//   e.g. "backup.7z (a3b2c1d0)"
//
// The UUID is generated once at entry creation and never changes.
// It makes entries unique even when two archives share the same basename.
// The basename is a searchable substring; the UUID disambiguates collisions.
// -------------------------------------------------------------------

// generateUUID8 generates a cryptographically random 8-character hex string.
func generateUUID8() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// makeEntryTitle constructs a KeePass entry title from a basename and uuid8.
// Result format: "basename (uuid8)"
func makeEntryTitle(basename, uuid8 string) string {
	return basename + " (" + uuid8 + ")"
}

// parseEntryTitle parses a title produced by makeEntryTitle.
// Returns (basename, uuid8, true) on success, ("", "", false) if the title
// does not match the expected format.
func parseEntryTitle(title string) (basename, uuid8 string, ok bool) {
	m := uuidTitleRe.FindStringSubmatch(title)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// -------------------------------------------------------------------
// Backward-compat encoded path helpers (kept for reading old entries)
//
// KeePass entry titles must not contain "/" (interpreted as group separator).
// Old entries used bijective %25+%2F encoding of the absolute archive path.
// New code does NOT write this format; it is only used in the lookup chain
// to remain compatible with databases created before the UUID migration.
// -------------------------------------------------------------------

// encodeArchivePath produces a flat KeePass-safe title from an absolute path.
// DEPRECATED for new entries — use makeEntryTitle instead.
func encodeArchivePath(absPath string) string {
	s := strings.ReplaceAll(absPath, "%", "%25") // escape existing %
	s = strings.ReplaceAll(s, "/", "%2F")        // escape path separator
	return s
}

// -------------------------------------------------------------------
// Multi-match types
// -------------------------------------------------------------------

// EntryCandidate represents a single KeePass entry match.
type EntryCandidate struct {
	// EntryPath is the full KeePass path (group/title) used as the lookup key.
	EntryPath string
	// Title is the entry's title (e.g. "backup.7z (a3b2c1d0)").
	Title string
	// LastKnownPath is the value stored in the Username field — the absolute
	// archive path at the time the entry was created/last updated.
	LastKnownPath string
}

// MultiMatchError is returned when more than one KeePass entry matches the
// archive basename. The caller should present Candidates to the user and let
// them pick one interactively.
type MultiMatchError struct {
	Basename   string
	Candidates []EntryCandidate
}

func (e *MultiMatchError) Error() string {
	return fmt.Sprintf("multiple KeePass entries found for '%s' (%d matches)", e.Basename, len(e.Candidates))
}

// IsMultiMatch reports whether err is a MultiMatchError.
func IsMultiMatch(err error) bool {
	_, ok := err.(*MultiMatchError)
	return ok
}

// -------------------------------------------------------------------
// Password lookup
// -------------------------------------------------------------------

// GetPasswordForArchive attempts to find the password for an archive.
// Lookup chain:
//  1. Search by basename (new UUID format):         "basename (uuid8)"
//     — Single hit → return immediately.
//     — Multiple hits → return MultiMatchError so caller can prompt.
//  2. Backward compat: encoded exact path           "%2Fhome%2F...%2Farchive.7z"
//  3. Backward compat: old flat basename            "archive.7z"
//  4. Split archive fallbacks (for each of the above):
//     a. Search by NormalizedName (new UUID format)
//     b. Encoded normalized path (backward compat)
//     c. Old flat normalized name (backward compat)
func GetPasswordForArchive(kp PasswordProvider, entryPathPrefix, archivePath string) ([]byte, string, error) {
	if kp == nil {
		return nil, "", fmt.Errorf("password provider is nil")
	}
	if archivePath == "" {
		return nil, "", fmt.Errorf("archive path is empty")
	}

	info := AnalyzeArchive(archivePath)

	absPath, err := filepath.Abs(archivePath)
	if err != nil {
		absPath = archivePath
	}

	prefix := entryPathPrefix
	tried := make([]string, 0, 6)

	buildPath := func(title string) string {
		if prefix == "" {
			return title
		}
		return prefix + "/" + title
	}

	tryPath := func(entryPath string) ([]byte, string, bool) {
		tried = append(tried, entryPath)
		pass, err := kp.GetPassword(entryPath)
		if err == nil {
			return pass, entryPath, true
		}
		return nil, "", false
	}

	// searchByBasename searches for entries matching the given basename.
	// It accepts both new UUID-format titles ("basename (uuid8)") and old-format
	// titles (encoded path or flat basename) found via keepassxc-cli search.
	//
	// keepassxc-cli search uses title-exact/prefix matching, not arbitrary substring.
	// For UUID-format titles ("test.7z (6031dd3b)") searching "test.7z" alone does not
	// match — we must also search "test.7z (" (including the space+paren prefix of the
	// UUID suffix) to reliably find UUID-format entries.
	// For old-format entries the Username field verifies the basename.
	searchByBasename := func(basename string) ([]byte, string, error) {
		// Search with two patterns — collect deduped results:
		//   1. "basename ("  → finds new UUID-format entries
		//   2. plain basename → finds old encoded-path and flat-basename entries
		seen := make(map[string]bool)
		var allResults []string
		for _, pattern := range []string{basename + " (", basename} {
			results, _ := kp.Search(pattern)
			for _, res := range results {
				res = filepath.ToSlash(res)
				if !seen[res] {
					seen[res] = true
					allResults = append(allResults, res)
				}
			}
		}

		// Filter to our group prefix and classify each result
		var candidates []EntryCandidate
		for _, res := range allResults {
			// Must be inside our group
			if prefix != "" && !strings.HasPrefix(res, filepath.ToSlash(prefix)+"/") {
				continue
			}

			// Extract title part (after group prefix)
			title := res
			if prefix != "" {
				title = strings.TrimPrefix(res, filepath.ToSlash(prefix)+"/")
			}

			// Case 1: new UUID format — basename must match exactly
			if b, _, ok := parseEntryTitle(title); ok {
				if b != basename {
					continue
				}
				lastKnownPath, _ := kp.GetAttribute(res, "Username")
				candidates = append(candidates, EntryCandidate{
					EntryPath:     res,
					Title:         title,
					LastKnownPath: lastKnownPath,
				})
				continue
			}

			// Case 2: old format (encoded path or flat basename) — verify via Username.
			// The search already found it as a substring match; confirm the basename
			// by checking the stored Username (last-known absolute path).
			lastKnownPath, err := kp.GetAttribute(res, "Username")
			if err == nil && filepath.Base(lastKnownPath) == basename {
				candidates = append(candidates, EntryCandidate{
					EntryPath:     res,
					Title:         title,
					LastKnownPath: lastKnownPath,
				})
				continue
			}

			// Old flat-basename format: title IS the basename (no path, no encoding)
			if title == basename {
				lastKnownPath, _ = kp.GetAttribute(res, "Username")
				candidates = append(candidates, EntryCandidate{
					EntryPath:     res,
					Title:         title,
					LastKnownPath: lastKnownPath,
				})
			}
		}

		switch len(candidates) {
		case 0:
			return nil, "", nil // not found via this method
		case 1:
			pass, entryPath, ok := tryPath(candidates[0].EntryPath)
			if ok {
				return pass, entryPath, nil
			}
			return nil, "", nil
		default:
			return nil, "", &MultiMatchError{Basename: basename, Candidates: candidates}
		}
	}

	// --- 1. New UUID format: search by basename ---
	searchTarget := filepath.Base(archivePath)
	pass, found, searchErr := searchByBasename(searchTarget)
	if searchErr != nil {
		return nil, "", searchErr // MultiMatchError — caller handles prompt
	}
	if pass != nil {
		return pass, found, nil
	}

	// --- 2. Backward compat: encoded exact path ---
	encodedPath := buildPath(encodeArchivePath(absPath))
	if pass, found, ok := tryPath(encodedPath); ok {
		return pass, found, nil
	}

	// --- 3. Backward compat: old flat basename ---
	oldPath := buildPath(filepath.Base(archivePath))
	if pass, found, ok := tryPath(oldPath); ok {
		return pass, found, nil
	}

	// --- 4. Split archive fallbacks ---
	if info.IsSplit {
		// 4a. New UUID format for normalized name
		pass, found, searchErr = searchByBasename(info.NormalizedName)
		if searchErr != nil {
			return nil, "", searchErr
		}
		if pass != nil {
			return pass, found, nil
		}

		// 4b. Encoded normalized path (backward compat)
		absNorm := filepath.Join(filepath.Dir(absPath), info.NormalizedName)
		encodedNorm := buildPath(encodeArchivePath(absNorm))
		if pass, found, ok := tryPath(encodedNorm); ok {
			return pass, found, nil
		}

		// 4c. Old flat normalized name (backward compat)
		oldNorm := buildPath(info.NormalizedName)
		if pass, found, ok := tryPath(oldNorm); ok {
			return pass, found, nil
		}
	}

	return nil, "", &PasswordNotFoundError{
		ArchiveName: info.OriginalName,
		Tried:       tried,
	}
}

// -------------------------------------------------------------------
// Interactive multi-match prompt
// -------------------------------------------------------------------

// promptMultiMatch presents numbered candidates to the user on stderr and
// reads a selection from stdin. Returns the chosen EntryCandidate.
// Returns an error if stdin is not a TTY or the user input is invalid.
func promptMultiMatch(candidates []EntryCandidate) (EntryCandidate, error) {
	fmt.Fprintf(os.Stderr, "\nFound %d entries:\n\n", len(candidates))
	for i, c := range candidates {
		fmt.Fprintf(os.Stderr, "  [%d]  %s\n", i+1, c.Title)
		if c.LastKnownPath != "" {
			fmt.Fprintf(os.Stderr, "       Last known path: %s\n", c.LastKnownPath)
		}
		fmt.Fprintln(os.Stderr)
	}
	fmt.Fprintf(os.Stderr, "Select [1-%d]: ", len(candidates))

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return EntryCandidate{}, fmt.Errorf("no input received")
	}
	line := strings.TrimSpace(scanner.Text())

	var idx int
	if _, err := fmt.Sscanf(line, "%d", &idx); err != nil || idx < 1 || idx > len(candidates) {
		return EntryCandidate{}, fmt.Errorf("invalid selection %q", line)
	}
	return candidates[idx-1], nil
}

// resolvePassword is a convenience wrapper that calls GetPasswordForArchive
// and handles MultiMatchError by prompting the user interactively.
// Returns (password, entryPath, needsMigration, err).
// needsMigration is true when the entry is in old format (non-UUID title) and
// the caller should call migrateEntry after a successful operation.
func resolvePassword(kp PasswordProvider, prefix, archivePath string) (password []byte, entryPath string, needsMigration bool, err error) {
	var resolvedPath string
	password, resolvedPath, err = GetPasswordForArchive(kp, prefix, archivePath)
	if err == nil {
		// Check if the resolved entry is already in UUID format
		title := resolvedPath
		if prefix != "" {
			title = strings.TrimPrefix(filepath.ToSlash(resolvedPath), filepath.ToSlash(prefix)+"/")
		}
		_, _, isUUID := parseEntryTitle(title)
		return password, resolvedPath, !isUUID, nil
	}

	mm, ok := err.(*MultiMatchError)
	if !ok {
		return nil, "", false, err
	}

	chosen, promptErr := promptMultiMatch(mm.Candidates)
	if promptErr != nil {
		return nil, "", false, fmt.Errorf("multiple entries for '%s': %w", mm.Basename, promptErr)
	}

	password, err = kp.GetPassword(chosen.EntryPath)
	if err != nil {
		return nil, "", false, fmt.Errorf("failed to get password for selected entry: %w", err)
	}
	_, _, isUUID := parseEntryTitle(chosen.Title)
	return password, chosen.EntryPath, !isUUID, nil
}

// -------------------------------------------------------------------
// Entry migration (old format → new UUID format)
// -------------------------------------------------------------------

// EntryMigrator extends PasswordProvider with the write operations needed
// to migrate a KeePass entry from old format to new UUID-keyed format.
type EntryMigrator interface {
	PasswordProvider
	AddEntry(group, title string, password []byte, username, url string) error
	DeleteEntry(entryPath string) error
}

// migrateEntry upgrades an old-format KeePass entry to the new UUID title format.
// It silently adds a new entry and deletes the old one.
// Failures are non-fatal; the caller should log a warning at most.
func migrateEntry(kp EntryMigrator, prefix, oldEntryPath string, password []byte, lastKnownPath string) (newEntryPath string, err error) {
	uuid8, err := generateUUID8()
	if err != nil {
		return "", fmt.Errorf("uuid generation: %w", err)
	}
	basename := filepath.Base(lastKnownPath)
	if basename == "" || basename == "." {
		basename = filepath.Base(oldEntryPath)
	}
	newTitle := makeEntryTitle(basename, uuid8)
	newEntryPath = prefix + "/" + newTitle
	if prefix == "" {
		newEntryPath = newTitle
	}

	if err := kp.AddEntry(prefix, newTitle, password, lastKnownPath, "https://github.com/lxstig/7zkpxc"); err != nil {
		return "", fmt.Errorf("add new entry: %w", err)
	}
	if err := kp.DeleteEntry(oldEntryPath); err != nil {
		// New entry already created — non-fatal, old entry stays as duplicate
		return newEntryPath, fmt.Errorf("delete old entry: %w", err)
	}
	return newEntryPath, nil
}

// -------------------------------------------------------------------
// PasswordProvider interface
// -------------------------------------------------------------------

// PasswordProvider is an interface for password retrieval
type PasswordProvider interface {
	GetPassword(key string) ([]byte, error)
	GetAttribute(entryPath, attribute string) (string, error)
	Search(query string) ([]string, error)
}

// -------------------------------------------------------------------
// Error types
// -------------------------------------------------------------------

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
