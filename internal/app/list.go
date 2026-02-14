package app

import (
	"fmt"

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

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	// Use the fallback strategy to find the password:
	// 1. Normalized name (e.g., archive.7z.001 -> archive.7z)
	// 2. Original filename
	// 3. Base name without extension (for split archives)
	fmt.Printf("Fetching password for '%s'...\n", archivePath)
	password, err := GetPasswordForArchive(kp, cfg.General.DefaultGroup, archivePath)
	if err != nil {
		if IsPasswordNotFound(err) {
			return fmt.Errorf("failed to get password (is the entry in '%s'?): %w", cfg.General.DefaultGroup, err)
		}
		return fmt.Errorf("failed to get password: %w", err)
	}

	// 7z l archive.7z — password is sent via PTY when 7z prompts for it
	sevenZipArgs := []string{"l", archivePath}

	err = sevenzip.Run(password, sevenZipArgs)
	if err != nil {
		return fmt.Errorf("list failed: %w", err)
	}

	return nil
}
