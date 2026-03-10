package app

import (
	"fmt"
	"path/filepath"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "l <archive_path>",
	Short:   "List contents of archive",
	Args:    cobra.ExactArgs(1),
	RunE:    runList,
	GroupID: "actions",
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(archivePath)
	if err != nil {
		absPath = archivePath
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	fmt.Printf("Fetching password for '%s'...\n", archivePath)
	password, entryPath, needsMigration, err := resolvePassword(kp, cfg.General.DefaultGroup, archivePath)
	if err != nil {
		if IsPasswordNotFound(err) {
			return fmt.Errorf("failed to get password (is the entry in '%s'?): %w", cfg.General.DefaultGroup, err)
		}
		return fmt.Errorf("failed to get password: %w", err)
	}

	// 7z l archive.7z — password is sent via PTY when 7z prompts for it
	fmt.Printf("Listing '%s'...\n", archivePath)
	runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, []string{"l", archivePath})

	if runErr == nil && needsMigration {
		// Migrate while password bytes are still valid (BEFORE zeroing)
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

	if runErr == nil {
		updatePathIfMoved(kp, entryPath, absPath)
	}

	// Zero password after all uses
	for i := range password {
		password[i] = 0
	}

	if runErr != nil {
		return fmt.Errorf("list failed: %w", runErr)
	}
	return nil
}
