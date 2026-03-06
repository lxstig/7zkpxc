package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:     "rename <old_archive_path> <new_archive_path>",
	Short:   "Rename or move an archive and update its KeePassXC entry",
	Long:    `Renames or moves an archive file on disk and updates the corresponding entry inside the KeePassXC database. Both the file system operation and the KeePass record are updated atomically: if any step fails the previous steps are rolled back.`,
	Args:    cobra.ExactArgs(2),
	RunE:    runRename,
	GroupID: "actions",
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldArchivePath := args[0]
	newArchivePath := args[1]

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// Resolve absolute paths upfront
	absOld, err := filepath.Abs(oldArchivePath)
	if err != nil {
		return fmt.Errorf("failed to resolve old archive path: %w", err)
	}

	absNew, err := filepath.Abs(newArchivePath)
	if err != nil {
		return fmt.Errorf("failed to resolve new archive path: %w", err)
	}

	// 1. Verify the source file actually exists on disk
	if _, err := os.Stat(absOld); os.IsNotExist(err) {
		return fmt.Errorf("source archive does not exist: %s", absOld)
	} else if err != nil {
		return fmt.Errorf("cannot access source archive: %w", err)
	}

	// 2. Ensure the destination does not already exist (prevent silent overwrites)
	if _, err := os.Stat(absNew); err == nil {
		return fmt.Errorf("destination already exists: %s", absNew)
	}

	// 3. Ensure the destination directory exists
	if err := os.MkdirAll(filepath.Dir(absNew), 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 4. Look up the KeePass entry (before touching the file)
	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	fmt.Printf("Locating KeePass entry for '%s'...\n", oldArchivePath)
	password, oldKeePassPath, err := GetPasswordForArchive(kp, cfg.General.DefaultGroup, oldArchivePath)
	if err != nil {
		return fmt.Errorf("could not find KeePass entry for '%s': %w", oldArchivePath, err)
	}
	defer func() {
		for i := range password {
			password[i] = 0
		}
	}()

	// 5. Rename the file on disk
	fmt.Printf("Moving '%s' → '%s'...\n", absOld, absNew)
	if err := os.Rename(absOld, absNew); err != nil {
		return fmt.Errorf("failed to move archive on disk: %w", err)
	}

	// 6. Add new KeePass entry (rollback file move on failure)
	newEntryTitle := filepath.Base(newArchivePath)
	fmt.Printf("Updating KeePassXC entry (title: %s)...\n", newEntryTitle)

	err = kp.AddEntry(
		cfg.General.DefaultGroup,
		newEntryTitle,
		password,
		absNew,                             // Username holds the absolute path
		"https://github.com/lxstig/7zkpxc", // URL
	)
	if err != nil {
		// Rollback: move the file back
		fmt.Printf("KeePass update failed, rolling back file move...\n")
		if rbErr := os.Rename(absNew, absOld); rbErr != nil {
			return fmt.Errorf("keepassxc-cli add failed: %w\nROLLBACK FAILED (file is at %s): %v", err, absNew, rbErr)
		}
		return fmt.Errorf("failed to create new KeePass entry (file move rolled back): %w", err)
	}

	// 7. Delete old KeePass entry (non-fatal: new entry and file are already correct)
	fmt.Printf("Cleaning up old KeePass entry ('%s')...\n", oldKeePassPath)
	if err := kp.DeleteEntry(oldKeePassPath); err != nil {
		fmt.Printf("Warning: could not delete old entry '%s': %v\n", oldKeePassPath, err)
		fmt.Println("The archive was moved and the new entry was created successfully.")
		fmt.Println("You may want to delete the old entry manually from KeePassXC.")
		return nil
	}

	fmt.Println("Done. Archive moved and KeePassXC entry updated.")
	return nil
}
