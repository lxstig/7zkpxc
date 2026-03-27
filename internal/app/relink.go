package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var relinkCmd = &cobra.Command{
	Use:   "relink <archive_or_directory>",
	Short: "Relink archives to their KeePassXC entries",
	Long: `Finds the correct KeePassXC entry for an archive by testing each
entry's password. Use this when an archive can't be opened because
it was renamed, moved, or restored after a fresh install.

Single archive:
  7zkpxc relink archive.7z

All archives in a directory:
  7zkpxc relink .
  7zkpxc relink ~/archives/`,
	Args:    cobra.ExactArgs(1),
	RunE:    runRelink,
	GroupID: "actions",
}

func init() {
	rootCmd.AddCommand(relinkCmd)
}

// relinkResult tracks the outcome of a single archive relink attempt.
type relinkResult struct {
	Archive string
	Status  string // "relinked", "verified", "no_match", "unencrypted", "error"
	Detail  string
}

// entryInfo holds pre-fetched entry data for size-filtered matching.
type entryInfo struct {
	EntryPath  string
	Title      string
	Username   string // last known archive path
	StoredSize int64  // from metadata; 0 = no metadata
}

func runRelink(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	target := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		absTarget = target
	}

	info, err := os.Stat(absTarget)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %w", target, err)
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	if info.IsDir() {
		return relinkDirectory(kp, cfg, absTarget)
	}
	return relinkSingleArchive(kp, cfg, absTarget)
}

// collectEntries gathers all entries from the group with their metadata.
func collectEntries(kp *keepass.Client, group string) ([]entryInfo, error) {
	titles, err := kp.ListEntries(group)
	if err != nil {
		return nil, fmt.Errorf("failed to list entries in '%s': %w", group, err)
	}

	var entries []entryInfo
	for _, title := range titles {
		entryPath := filepath.ToSlash(filepath.Clean(group + "/" + title))

		username, _ := kp.GetAttribute(entryPath, "Username")
		notes, _ := kp.GetAttribute(entryPath, "Notes")
		meta := parseMetadata(notes)

		entries = append(entries, entryInfo{
			EntryPath:  entryPath,
			Title:      title,
			Username:   username,
			StoredSize: meta.Size,
		})
	}
	return entries, nil
}

// partitionBySize splits entries into three groups:
//   - candidates: stored size matches archive size
//   - noMetadata: no stored size (old entries, backward compat)
//   - mismatch:   stored size differs (stale metadata, native 7z update)
func partitionBySize(entries []entryInfo, archiveSize int64) (candidates, noMetadata, mismatch []entryInfo) {
	for _, e := range entries {
		switch {
		case e.StoredSize == 0 && archiveSize == 0:
			// Both zero — treat as no-metadata (0-byte archive is invalid anyway)
			noMetadata = append(noMetadata, e)
		case e.StoredSize == archiveSize:
			candidates = append(candidates, e)
		case e.StoredSize == 0:
			noMetadata = append(noMetadata, e)
		default:
			mismatch = append(mismatch, e)
		}
	}
	return
}

func relinkSingleArchive(kp *keepass.Client, cfg *config.Config, absArchivePath string) error {
	basename := filepath.Base(absArchivePath)

	archiveInfo, err := os.Stat(absArchivePath)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %w", basename, err)
	}
	archiveSize := archiveInfo.Size()

	fmt.Printf("Scanning '%s' for entries...\n", cfg.General.DefaultGroup)
	entries, err := collectEntries(kp, cfg.General.DefaultGroup)
	if err != nil {
		return err
	}

	candidates, noMetadata, mismatch := partitionBySize(entries, archiveSize)

	fmt.Printf("Found %d entries (%d size-matched, %d no metadata, %d size-mismatched).\n",
		len(entries), len(candidates), len(noMetadata), len(mismatch))
	fmt.Printf("Testing passwords against '%s'...\n\n", basename)

	// Phase 1: size-matched candidates (fastest, most likely)
	if result := tryVerifyEntries(kp, cfg, absArchivePath, candidates); result != nil {
		return nil
	}

	// Phase 2: entries without metadata (old entries)
	if len(noMetadata) > 0 {
		fmt.Printf("  Trying %d entries without metadata...\n", len(noMetadata))
		if result := tryVerifyEntries(kp, cfg, absArchivePath, noMetadata); result != nil {
			return nil
		}
	}

	// Phase 3: size-mismatched entries (stale metadata — archive modified outside 7zkpxc)
	if len(mismatch) > 0 {
		fmt.Printf("  Trying %d entries with different stored size (stale metadata?)...\n", len(mismatch))
		if result := tryVerifyEntries(kp, cfg, absArchivePath, mismatch); result != nil {
			return nil
		}
	}

	fmt.Printf("\n✗ No matching entry found for '%s'.\n", basename)
	fmt.Println("  This archive may not be managed by 7zkpxc, or the entry may have been deleted.")
	return nil
}

// tryVerifyEntries attempts VerifyPassword on each entry. Returns non-nil on match or unencrypted.
func tryVerifyEntries(kp *keepass.Client, cfg *config.Config, absArchivePath string, entries []entryInfo) *entryInfo {
	basename := filepath.Base(absArchivePath)

	for i, e := range entries {
		fmt.Printf("  [%d/%d] %s — ", i+1, len(entries), e.Title)

		password, err := kp.GetPassword(e.EntryPath)
		if err != nil {
			fmt.Printf("✗ (could not get password)\n")
			continue
		}

		matched, verifyErr := sevenzip.VerifyPassword(cfg.SevenZip.BinaryPath, password, absArchivePath)
		for j := range password {
			password[j] = 0
		}

		if verifyErr != nil {
			fmt.Printf("✗\n")
			continue
		}
		if !matched {
			fmt.Printf("✗ (unencrypted archive)\n")
			fmt.Printf("\n⚠ Archive '%s' is not encrypted — cannot match to a KeePassXC entry.\n", basename)
			return &e // signal to stop
		}

		// Match!
		fmt.Printf("✓ MATCH\n")

		// Check if already correctly linked
		if e.Username == absArchivePath {
			fmt.Printf("\n✓ '%s' is already correctly linked.\n", basename)
			return &e
		}

		// Relink
		if err := relinkEntry(kp, cfg, e.EntryPath, e.Title, absArchivePath); err != nil {
			fmt.Printf("  ⚡ Relink failed: %v\n", err)
		}
		return &e
	}
	return nil
}

func relinkDirectory(kp *keepass.Client, cfg *config.Config, absDir string) error {
	archives, err := findArchivesInDir(absDir)
	if err != nil {
		return err
	}
	if len(archives) == 0 {
		fmt.Printf("No .7z archives found in '%s'.\n", absDir)
		return nil
	}

	fmt.Printf("Scanning '%s' for entries...\n", cfg.General.DefaultGroup)
	entries, err := collectEntries(kp, cfg.General.DefaultGroup)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d archives, %d entries.\n\n", len(archives), len(entries))

	var results []relinkResult
	for i, archivePath := range archives {
		basename := filepath.Base(archivePath)
		fmt.Printf("[%d/%d] %s\n", i+1, len(archives), basename)

		result, remaining := tryMatchArchive(kp, cfg, archivePath, entries)
		entries = remaining
		results = append(results, result)
	}

	printRelinkSummary(results)
	return nil
}

// tryMatchArchive attempts to match a single archive against entries using size filter.
func tryMatchArchive(kp *keepass.Client, cfg *config.Config, archivePath string, entries []entryInfo) (relinkResult, []entryInfo) {
	basename := filepath.Base(archivePath)

	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		fmt.Printf("  ✗ Cannot stat: %v\n\n", err)
		return relinkResult{basename, "error", err.Error()}, entries
	}

	candidates, noMetadata, mismatch := partitionBySize(entries, archiveInfo.Size())

	// Phase 1: size-matched candidates
	if r, updated, ok := verifyAndRelink(kp, cfg, archivePath, candidates, entries); ok {
		return r, updated
	}

	// Phase 2: no metadata (old entries)
	if r, updated, ok := verifyAndRelink(kp, cfg, archivePath, noMetadata, entries); ok {
		return r, updated
	}

	// Phase 3: size-mismatched (stale metadata)
	if r, updated, ok := verifyAndRelink(kp, cfg, archivePath, mismatch, entries); ok {
		return r, updated
	}

	fmt.Printf("  ✗ No match found\n\n")
	return relinkResult{basename, "no_match", ""}, entries
}

// verifyAndRelink tries each entry's password against the archive.
// Returns (result, updatedEntries, matched). If matched is false, no match was found in this batch.
func verifyAndRelink(kp *keepass.Client, cfg *config.Config, archivePath string, batch, allEntries []entryInfo) (relinkResult, []entryInfo, bool) {
	basename := filepath.Base(archivePath)

	for _, e := range batch {
		password, err := kp.GetPassword(e.EntryPath)
		if err != nil {
			continue
		}

		matched, verifyErr := sevenzip.VerifyPassword(cfg.SevenZip.BinaryPath, password, archivePath)
		for j := range password {
			password[j] = 0
		}

		if verifyErr != nil {
			continue
		}
		if !matched {
			fmt.Printf("  ⚠ Unencrypted archive — skipped\n\n")
			return relinkResult{basename, "unencrypted", ""}, allEntries, true
		}

		if e.Username == archivePath {
			fmt.Printf("  Already linked & verified — skipped\n\n")
			return relinkResult{basename, "verified", ""}, allEntries, true
		}

		if err := relinkEntry(kp, cfg, e.EntryPath, e.Title, archivePath); err != nil {
			fmt.Printf("  ⚡ Relink failed: %v\n\n", err)
			return relinkResult{basename, "error", err.Error()}, allEntries, true
		}
		fmt.Printf("  ✓ Relinked: %s\n\n", e.Title)
		return relinkResult{basename, "relinked", e.Title}, removeEntry(allEntries, e.EntryPath), true
	}
	return relinkResult{}, nil, false
}

// relinkEntry updates a KeePass entry to point to the new archive path.
// Silent — prints nothing on success (caller handles output).
func relinkEntry(kp *keepass.Client, cfg *config.Config, entryPath, title, absArchivePath string) error {
	newBasename := filepath.Base(absArchivePath)

	_, uuid8, ok := parseEntryTitle(title)
	var newTitle string
	if ok {
		newTitle = makeEntryTitle(newBasename, uuid8)
	} else {
		newUUID, err := generateUniqueUUID8(kp, cfg.General.DefaultGroup, newBasename)
		if err != nil {
			newTitle = title
		} else {
			newTitle = makeEntryTitle(newBasename, newUUID)
		}
	}

	if err := kp.EditEntryTitle(entryPath, newTitle, absArchivePath); err != nil {
		return fmt.Errorf("keepassxc edit failed: %w", err)
	}

	// Update metadata on the relinked entry
	newEntryPath := filepath.ToSlash(filepath.Clean(cfg.General.DefaultGroup + "/" + newTitle))
	updateMetadata(kp, newEntryPath, absArchivePath)
	return nil
}

// findArchivesInDir returns paths to all .7z files (including first split volume) in a directory.
func findArchivesInDir(dir string) ([]string, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var archives []string
	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		// Match .7z and .7z.001 (first split volume only — .002+ have no header)
		if strings.HasSuffix(lower, ".7z") || strings.HasSuffix(lower, ".7z.001") {
			archives = append(archives, filepath.Join(dir, e.Name()))
		}
	}
	return archives, nil
}

// removeEntry returns a new slice with the specified entry path removed.
func removeEntry(entries []entryInfo, entryPath string) []entryInfo {
	result := make([]entryInfo, 0, len(entries)-1)
	for _, e := range entries {
		if e.EntryPath != entryPath {
			result = append(result, e)
		}
	}
	return result
}

func printRelinkSummary(results []relinkResult) {
	var relinked, verified, noMatch, unencrypted, errored []string
	for _, r := range results {
		switch r.Status {
		case "relinked":
			relinked = append(relinked, r.Archive)
		case "verified":
			verified = append(verified, r.Archive)
		case "no_match":
			noMatch = append(noMatch, r.Archive)
		case "unencrypted":
			unencrypted = append(unencrypted, r.Archive)
		case "error":
			errored = append(errored, r.Archive)
		}
	}

	fmt.Println("─────────────────────────────────")
	fmt.Println("Relink Summary:")
	if len(relinked) > 0 {
		fmt.Printf("  ✓ Relinked:    %d  (%s)\n", len(relinked), strings.Join(relinked, ", "))
	}
	if len(verified) > 0 {
		fmt.Printf("  ─ Verified:    %d  (already linked)\n", len(verified))
	}
	if len(noMatch) > 0 {
		fmt.Printf("  ✗ No match:    %d  (%s)\n", len(noMatch), strings.Join(noMatch, ", "))
	}
	if len(unencrypted) > 0 {
		fmt.Printf("  ⚠ Unencrypted: %d  (%s)\n", len(unencrypted), strings.Join(unencrypted, ", "))
	}
	if len(errored) > 0 {
		fmt.Printf("  ⚡ Errors:      %d  (%s)\n", len(errored), strings.Join(errored, ", "))
	}
	fmt.Println("─────────────────────────────────")
}
