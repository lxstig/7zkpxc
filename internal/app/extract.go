package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:     "x <archive_path>",
	Short:   "Extract archive contents",
	Args:    cobra.ExactArgs(1),
	RunE:    runExtract,
	GroupID: "actions",
}

func init() {
	extractCmd.Flags().StringP("output", "o", "", "Output directory for extracted files")
	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	// Use the fallback strategy to find the password:
	// 1. Normalized name (e.g., archive.7z.001 -> archive.7z)
	// 2. Original filename
	// 3. Base name without extension (for split archives)
	fmt.Printf("Fetching password for '%s'...\n", archivePath)
	password, entryPath, needsMigration, err := resolvePassword(kp, cfg.General.DefaultGroup, archivePath)
	if err != nil {
		if IsPasswordNotFound(err) {
			return fmt.Errorf("failed to get password (is the entry in '%s'?): %w", cfg.General.DefaultGroup, err)
		}
		return fmt.Errorf("failed to get password: %w", err)
	}

	// Build 7z extract args
	fmt.Printf("Extracting '%s'...\n", archivePath)
	sevenZipArgs := []string{"x", archivePath}

	outputDir, _ := cmd.Flags().GetString("output")
	if outputDir != "" {
		sevenZipArgs = append(sevenZipArgs, "-o"+outputDir)
	}

	runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs)

	if runErr == nil && needsMigration {
		// Migrate while password bytes are still valid (BEFORE zeroing)
		lastKnownPath := entryPath
		if lk, e := kp.GetAttribute(entryPath, "Username"); e == nil && lk != "" {
			lastKnownPath = lk
		}
		if _, e := migrateEntry(kp, cfg.General.DefaultGroup, entryPath, password, lastKnownPath); e != nil {
			fmt.Printf("Note: could not migrate entry to new format: %v\n", e)
		} else {
			fmt.Println("(Entry migrated to new format.)")
		}
	}

	// Zero password after all uses
	for i := range password {
		password[i] = 0
	}

	if runErr != nil {
		return fmt.Errorf("extraction failed: %w", runErr)
	}

	fmt.Println("Success! Archive extracted.")
	return nil
}
