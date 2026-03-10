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
	Use:   "a <archive_name> [files...]",
	Short: "Create encrypted archive or add files to an existing one",
	Long: `Creates a new encrypted archive with a unique password stored in KeePassXC.

If the archive already exists, retrieves its password from KeePassXC and
appends the provided files without creating a new entry.`,
	Args:    cobra.MinimumNArgs(2),
	RunE:    runAdd,
	GroupID: "actions",
}

func init() {
	// Compression flags — mutually exclusive
	addCmd.Flags().Bool("fast", false, "Fastest compression (-mx=1)")
	addCmd.Flags().Bool("best", false, "Best compression (-mx=9)")
	addCmd.MarkFlagsMutuallyExclusive("fast", "best")

	// Volume flag (only meaningful for new archives)
	addCmd.Flags().String("volume", "", "Create volumes, e.g. 100m, 1g (new archives only)")

	// Pass-through unknown flags to 7z (e.g. -sfx, -m0=lzma2)
	addCmd.FParseErrWhitelist.UnknownFlags = true

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archiveName := args[0]

	// Ensure archive extension
	if filepath.Ext(archiveName) == "" {
		archiveName += ".7z"
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	// Separate positional files from pass-through 7z flags
	var files, extraFlags []string
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			if _, err := os.Stat(arg); err == nil {
				// File exists but starts with "-" — security: reject to prevent injection
				return fmt.Errorf("security policy: archiving files starting with '-' is not supported to prevent parameter injection: %s", arg)
			}
			extraFlags = append(extraFlags, arg)
		} else {
			files = append(files, arg)
		}
	}

	// Dispatch based on whether the archive already exists
	if _, err := os.Stat(archiveName); err == nil {
		return runAddUpdate(cmd, cfg, kp, archiveName, files, extraFlags)
	}
	return runAddCreate(cmd, cfg, kp, archiveName, files, extraFlags)
}

// runAddCreate generates a new password, saves it to KeePassXC, and creates the archive.
// Rolls back the KeePass entry if 7z fails to avoid orphan entries.
func runAddCreate(
	cmd *cobra.Command,
	cfg *config.Config,
	kp *keepass.Client,
	archiveName string,
	files, extraFlags []string,
) error {
	// 1. Generate password
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
	//
	//    Title = "basename (uuid8)" — path-independent, UUID ensures uniqueness
	//    even when two archives share the same filename.
	fmt.Printf("Saving entry to KeePassXC (%s)...\n", cfg.General.KdbxPath)
	absArchivePath, err := filepath.Abs(archiveName)
	if err != nil {
		return fmt.Errorf("failed to resolve archive path: %w", err)
	}

	uuid8, err := generateUniqueUUID8(kp, cfg.General.DefaultGroup, filepath.Base(absArchivePath))
	if err != nil {
		return fmt.Errorf("failed to generate entry UUID: %w", err)
	}
	entryTitle := makeEntryTitle(filepath.Base(absArchivePath), uuid8)
	keePassEntryPath := filepath.ToSlash(filepath.Clean(cfg.General.DefaultGroup + "/" + entryTitle))

	if err := kp.AddEntry(
		cfg.General.DefaultGroup,
		entryTitle,
		password,
		absArchivePath, // Username — human-readable path for reference
		"https://github.com/lxstig/7zkpxc",
	); err != nil {
		return fmt.Errorf("failed to add entry to KeePassXC: %w", err)
	}

	// 3. Build 7z create arguments
	fmt.Printf("Creating archive '%s'...\n", archiveName)
	sevenZipArgs := buildCompressionArgs(cmd, cfg.SevenZip.DefaultArgs)
	sevenZipArgs = append(sevenZipArgs, "-p") // prompt for password (sent via PTY)
	sevenZipArgs = append(sevenZipArgs, archiveName)
	sevenZipArgs = append(sevenZipArgs, files...)
	sevenZipArgs = append(sevenZipArgs, extraFlags...)

	// 4. Run 7z — rollback KeePass entry on failure
	if err := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); err != nil {
		fmt.Println("Archive creation failed, rolling back KeePassXC entry...")
		if rbErr := kp.DeleteEntry(keePassEntryPath); rbErr != nil {
			fmt.Printf("Warning: rollback failed — manually delete '%s' from KeePassXC: %v\n", keePassEntryPath, rbErr)
		} else {
			fmt.Println("KeePassXC entry rolled back successfully.")
		}
		return fmt.Errorf("archive creation failed: %w", err)
	}

	return nil
}

// runAddUpdate retrieves the existing password from KeePassXC and appends
// files to the already-encrypted archive. No new KeePass entry is created.
func runAddUpdate(
	cmd *cobra.Command,
	cfg *config.Config,
	kp *keepass.Client,
	archiveName string,
	files, extraFlags []string,
) error {
	fmt.Printf("Archive '%s' already exists — fetching password from KeePassXC...\n", archiveName)

	password, entryPath, needsMigration, err := resolvePassword(kp, cfg.General.DefaultGroup, archiveName)
	if err != nil {
		if IsPasswordNotFound(err) {
			return fmt.Errorf("archive '%s' exists on disk but has no entry in KeePassXC.\n"+
				"If you created it outside 7zkpxc, you cannot use this command to update it", archiveName)
		}
		return fmt.Errorf("failed to fetch password: %w", err)
	}
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()

	// Build 7z update arguments.
	// Do NOT pass default_args (e.g. -mhe=on) — the archive already has its
	// encryption settings; re-specifying them may conflict.
	// Do NOT pass -p — 7z detects the existing encryption and prompts itself.
	fmt.Printf("Updating archive '%s'...\n", archiveName)
	sevenZipArgs := []string{"a"}

	// Honor compression level override if explicitly set
	fast, _ := cmd.Flags().GetBool("fast")
	best, _ := cmd.Flags().GetBool("best")
	if fast {
		sevenZipArgs = append(sevenZipArgs, "-mx=1")
	} else if best {
		sevenZipArgs = append(sevenZipArgs, "-mx=9")
	}

	sevenZipArgs = append(sevenZipArgs, archiveName)
	sevenZipArgs = append(sevenZipArgs, files...)
	sevenZipArgs = append(sevenZipArgs, extraFlags...)

	if err := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); err != nil {
		return fmt.Errorf("failed to update archive: %w", err)
	}

	// Migrate old-format entry while password is still valid (defer zeroes later)
	if needsMigration {
		lastKnownPath := entryPath
		if lk, e := kp.GetAttribute(entryPath, "Username"); e == nil && lk != "" {
			lastKnownPath = lk
		}
		var migrateErr error
		entryPath, migrateErr = migrateEntry(kp, cfg.General.DefaultGroup, entryPath, password, lastKnownPath)
		if migrateErr != nil {
			fmt.Printf("Note: could not migrate entry to new format: %v\n", migrateErr)
		} else {
			fmt.Println("(Entry migrated to new format.)")
		}
	}

	absArchiveName, err := filepath.Abs(archiveName)
	if err != nil {
		absArchiveName = archiveName
	}
	updatePathIfMoved(kp, entryPath, absArchiveName)

	fmt.Println("Files added to existing archive successfully.")
	return nil
}

// buildCompressionArgs builds the 7z argument list for a new archive.
func buildCompressionArgs(cmd *cobra.Command, defaultArgs []string) []string {
	args := []string{"a"}
	args = append(args, defaultArgs...)

	fast, _ := cmd.Flags().GetBool("fast")
	best, _ := cmd.Flags().GetBool("best")
	volume, _ := cmd.Flags().GetString("volume")

	if fast {
		args = append(args, "-mx=1")
	} else if best {
		args = append(args, "-mx=9")
	}
	if volume != "" {
		args = append(args, "-v"+volume)
	}

	return args
}
