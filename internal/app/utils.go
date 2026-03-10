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

// generateUniqueUUID8 generates a UUID8 that is guaranteed not to collide with
// an existing entry in the KeePass database under the given group.
// A collision is astronomically unlikely (~1 in 4 billion per call), but this
// loop provides mathematical certainty at virtually zero cost.
func generateUniqueUUID8(kp PasswordProvider, group, basename string) (string, error) {
	for {
		uuid8, err := generateUUID8()
		if err != nil {
			return "", err
		}
		title := makeEntryTitle(basename, uuid8)
		candidatePath := filepath.ToSlash(filepath.Clean(group + "/" + title))
		results, _ := kp.Search(title)
		collision := false
		for _, r := range results {
			if filepath.ToSlash(r) == candidatePath {
				collision = true
				break
			}
		}
		if !collision {
			return uuid8, nil
		}
	}
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

// collectCandidates searches keepassxc for entries matching basename under
// the given group prefix. It handles both new UUID-format titles
// ("basename (uuid8)") and old-format titles (encoded path, flat basename).
//
// keepassxc-cli search does prefix/exact title matching rather than arbitrary
// substring, so two search patterns are tried:
//   - "basename ("  → matches UUID-format titles
//   - "basename"    → matches old encoded-path and flat-basename titles
//
// For old-format results the Username field is used to confirm the basename.
func collectCandidates(kp PasswordProvider, prefix, basename string) ([]EntryCandidate, error) {
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

	prefixSlash := filepath.ToSlash(prefix) + "/"
	var candidates []EntryCandidate
	for _, res := range allResults {
		if prefix != "" && !strings.HasPrefix(res, prefixSlash) {
			continue
		}
		title := res
		if prefix != "" {
			title = strings.TrimPrefix(res, prefixSlash)
		}
		if c, ok := classifyCandidate(kp, res, title, basename); ok {
			candidates = append(candidates, c)
		}
	}
	return candidates, nil
}

// classifyCandidate decides whether a keepassxc search result belongs to the
// target basename and returns an EntryCandidate if it does.
func classifyCandidate(kp PasswordProvider, entryPath, title, basename string) (EntryCandidate, bool) {
	// New UUID format: title = "basename (uuid8)"
	if b, _, ok := parseEntryTitle(title); ok {
		if b != basename {
			return EntryCandidate{}, false
		}
		lastKnownPath, _ := kp.GetAttribute(entryPath, "Username")
		return EntryCandidate{EntryPath: entryPath, Title: title, LastKnownPath: lastKnownPath}, true
	}

	// Old format: verify via Username (last-known absolute path)
	lastKnownPath, err := kp.GetAttribute(entryPath, "Username")
	if err == nil && filepath.Base(lastKnownPath) == basename {
		return EntryCandidate{EntryPath: entryPath, Title: title, LastKnownPath: lastKnownPath}, true
	}

	// Oldest flat-basename format: title IS the basename
	if title == basename {
		lastKnownPath, _ = kp.GetAttribute(entryPath, "Username")
		return EntryCandidate{EntryPath: entryPath, Title: title, LastKnownPath: lastKnownPath}, true
	}
	return EntryCandidate{}, false
}

// passwordLookup holds state for a single GetPasswordForArchive call and
// provides helper methods to keep the main function's cyclomatic complexity low.
type passwordLookup struct {
	kp     PasswordProvider
	prefix string
	tried  []string
}

func (l *passwordLookup) buildPath(title string) string {
	if l.prefix == "" {
		return title
	}
	return l.prefix + "/" + title
}

func (l *passwordLookup) tryPath(entryPath string) ([]byte, string, bool) {
	l.tried = append(l.tried, entryPath)
	pass, err := l.kp.GetPassword(entryPath)
	if err == nil {
		return pass, entryPath, true
	}
	return nil, "", false
}

func (l *passwordLookup) searchAndTry(basename string) ([]byte, string, error) {
	candidates, _ := collectCandidates(l.kp, l.prefix, basename)
	switch len(candidates) {
	case 0:
		return nil, "", nil
	case 1:
		if pass, found, ok := l.tryPath(candidates[0].EntryPath); ok {
			return pass, found, nil
		}
		return nil, "", nil
	default:
		return nil, "", &MultiMatchError{Basename: basename, Candidates: candidates}
	}
}

// GetPasswordForArchive attempts to find the password for an archive.
// Lookup chain:
//  1. Search by basename (UUID format primary, old format fallback via collectCandidates)
//  2. Backward compat: encoded exact path  "%2Fhome%2F...%2Farchive.7z"
//  3. Backward compat: old flat basename   "archive.7z"
//  4. Split archive fallbacks for each of the above
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

	l := &passwordLookup{kp: kp, prefix: entryPathPrefix}

	// 1. Search by basename (UUID format primary, old format fallback)
	if pass, found, err := l.searchAndTry(filepath.Base(archivePath)); err != nil || pass != nil {
		return pass, found, err
	}

	// 2. Backward compat: encoded exact path
	if pass, found, ok := l.tryPath(l.buildPath(encodeArchivePath(absPath))); ok {
		return pass, found, nil
	}

	// 3. Backward compat: old flat basename
	if pass, found, ok := l.tryPath(l.buildPath(filepath.Base(archivePath))); ok {
		return pass, found, nil
	}

	// 4. Split archive fallbacks
	if info.IsSplit {
		return l.splitFallbacks(info, absPath)
	}

	return nil, "", &PasswordNotFoundError{ArchiveName: info.OriginalName, Tried: l.tried}
}

func (l *passwordLookup) splitFallbacks(info ArchiveInfo, absPath string) ([]byte, string, error) {
	if pass, found, err := l.searchAndTry(info.NormalizedName); err != nil || pass != nil {
		return pass, found, err
	}
	absNorm := filepath.Join(filepath.Dir(absPath), info.NormalizedName)
	if pass, found, ok := l.tryPath(l.buildPath(encodeArchivePath(absNorm))); ok {
		return pass, found, nil
	}
	if pass, found, ok := l.tryPath(l.buildPath(info.NormalizedName)); ok {
		return pass, found, nil
	}
	return nil, "", &PasswordNotFoundError{ArchiveName: info.OriginalName, Tried: l.tried}
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
	basename := filepath.Base(lastKnownPath)
	if basename == "" || basename == "." {
		basename = filepath.Base(oldEntryPath)
	}
	uuid8, err := generateUniqueUUID8(kp, prefix, basename)
	if err != nil {
		return "", fmt.Errorf("uuid generation: %w", err)
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
	UpdateEntryUsername(entryPath, username string) error
}

// -------------------------------------------------------------------
// Path staleness detection and update
// -------------------------------------------------------------------

// updatePathIfMoved checks whether the Username field (last known path) in the
// KeePass entry matches absArchivePath. If they differ, it silently updates
// the Username field so the GUI and CLI both show the current location.
// Always non-fatal: failures are printed as a Note, never returned.
func updatePathIfMoved(kp PasswordProvider, entryPath, absArchivePath string) {
	current, err := kp.GetAttribute(entryPath, "Username")
	if err != nil || current == absArchivePath {
		return
	}
	if err := kp.UpdateEntryUsername(entryPath, absArchivePath); err != nil {
		fmt.Printf("Note: could not update location in KeePassXC: %v\n", err)
		return
	}
	fmt.Println("(Location updated in KeePassXC.)")
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
