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

var addCmd = &cobra.Command{
	Use:     "a <archive_name> [files...]",
	Short:   "Add files to archive",
	Args:    cobra.MinimumNArgs(2),
	RunE:    runAdd,
	GroupID: "actions",
}

func init() {
	// Compression Flags
	addCmd.Flags().Bool("fast", false, "Fastest compression (-mx=1)")
	addCmd.Flags().Bool("best", false, "Best compression (-mx=9)")
	addCmd.MarkFlagsMutuallyExclusive("fast", "best")

	// Volume Flags
	addCmd.Flags().String("volume", "", "Create volumes (e.g. 100m, 1g)")

	// Allow unknown flags to pass through to 7z (e.g. -sfx, -m0=lzma2)
	addCmd.FParseErrWhitelist.UnknownFlags = true

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archiveName := args[0]
	var files []string

	// Ensure archive extension
	if filepath.Ext(archiveName) == "" {
		archiveName += ".7z"
	}

	// 0. Pre-flight: refuse to overwrite an existing archive
	if _, err := os.Stat(archiveName); err == nil {
		return fmt.Errorf("archive '%s' already exists — delete or rename it first", archiveName)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	// 1. Generate Password
	fmt.Printf("Generating %d-character secure password...\n", cfg.General.PasswordLength)
	password, err := kp.GeneratePassword(cfg.General.PasswordLength)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()

	// 2. Save to KeePassXC BEFORE creating the archive.
	//    If 7z fails we can roll this back cleanly. The reverse is harder:
	//    an archive with no KeePass entry is silently lost data.
	fmt.Printf("Saving entry to KeePassXC (%s)...\n", cfg.General.KdbxPath)
	absArchivePath, err := filepath.Abs(archiveName)
	if err != nil {
		return fmt.Errorf("failed to resolve archive path: %w", err)
	}

	keePassEntryPath, err := addEntryWithCollisionResolution(
		kp,
		cfg.General.DefaultGroup,
		filepath.Base(archiveName),
		password,
		absArchivePath,
	)
	if err != nil {
		return fmt.Errorf("failed to add entry to KeePassXC: %w", err)
	}

	// 3. Build 7z arguments
	fmt.Printf("Creating archive '%s'...\n", archiveName)

	sevenZipArgs := []string{"a"}

	// Default args from config
	sevenZipArgs = append(sevenZipArgs, cfg.SevenZip.DefaultArgs...)

	// Handle custom flags
	fast, _ := cmd.Flags().GetBool("fast")
	best, _ := cmd.Flags().GetBool("best")
	volume, _ := cmd.Flags().GetString("volume")

	if fast {
		sevenZipArgs = append(sevenZipArgs, "-mx=1")
	} else if best {
		sevenZipArgs = append(sevenZipArgs, "-mx=9")
	}

	if volume != "" {
		sevenZipArgs = append(sevenZipArgs, "-v"+volume)
	}

	// Separate positional files from pass-through 7z flags (starting with "-")
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			if _, err := os.Stat(arg); err == nil {
				// File exists but starts with "-" — security: reject to prevent injection
				return fmt.Errorf("security policy: archiving files starting with '-' is not supported to prevent parameter injection: %s", arg)
			}
			sevenZipArgs = append(sevenZipArgs, arg)
		} else {
			files = append(files, arg)
		}
	}

	// Tell 7z to prompt for password (handled securely via PTY)
	sevenZipArgs = append(sevenZipArgs, "-p")
	sevenZipArgs = append(sevenZipArgs, archiveName)
	sevenZipArgs = append(sevenZipArgs, files...)

	// 4. Run 7z — if this fails, roll back the KeePass entry so there's no orphan
	if err := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); err != nil {
		fmt.Println("Archive creation failed, rolling back KeePassXC entry...")
		if rbErr := kp.DeleteEntry(keePassEntryPath); rbErr != nil {
			fmt.Printf("Warning: rollback failed, you may need to manually delete '%s' from KeePassXC: %v\n", keePassEntryPath, rbErr)
		} else {
			fmt.Println("KeePassXC entry rolled back successfully.")
		}
		return fmt.Errorf("archive creation failed: %w", err)
	}

	return nil
}

// addEntryWithCollisionResolution attempts to add a KeePass entry to the flat
// DefaultGroup. If an entry with the same title already exists it distinguishes
// two cases:
//
//  1. Same absolute path (absPath) in the Username field → already registered,
//     return a clear error so the caller can inform the user.
//  2. Different path → real collision (same filename, different directory).
//     Fall back to DefaultGroup/<parentDirName>/<title> (one extra level only).
//
// Returns the final KeePass entry path so the caller can roll it back if needed.
func addEntryWithCollisionResolution(
	kp *keepass.Client,
	defaultGroup, title string,
	password []byte,
	absPath string,
) (string, error) {
	flatPath := filepath.ToSlash(filepath.Clean(defaultGroup + "/" + title))

	err := kp.AddEntry(defaultGroup, title, password, absPath, "https://github.com/lxstig/7zkpxc")
	if err == nil {
		return flatPath, nil
	}

	// Only attempt collision resolution for "already exists" errors
	if !strings.Contains(err.Error(), "already exists") {
		return "", err
	}

	// Check whose archive occupies the flat path
	existing, attrErr := kp.GetAttribute(flatPath, "Username")
	if attrErr == nil && existing == absPath {
		return "", fmt.Errorf("archive '%s' is already registered in KeePassXC at '%s'", title, flatPath)
	}

	// Real collision: same filename, different directory.
	// Use immediate parent directory name as a single disambiguating sub-group.
	parentDir := filepath.Base(filepath.Dir(absPath))
	collisionGroup := filepath.ToSlash(filepath.Clean(defaultGroup + "/" + parentDir))
	collisionPath := collisionGroup + "/" + title

	fmt.Printf("Note: '%s' already exists in KeePassXC — storing under '%s' to avoid collision.\n", title, collisionGroup)

	if err2 := kp.AddEntry(collisionGroup, title, password, absPath, "https://github.com/lxstig/7zkpxc"); err2 != nil {
		return "", fmt.Errorf("failed to add entry even after collision resolution: %w", err2)
	}

	return collisionPath, nil
}
