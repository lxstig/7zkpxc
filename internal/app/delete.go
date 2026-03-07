package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "d <archive_path>",
	Short:   "Delete KeePassXC entry for an archive",
	Long:    `Removes the password entry stored in KeePassXC for the given archive. This does NOT delete the archive file itself.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDelete,
	GroupID: "actions",
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	// Reuse GetPasswordForArchive for consistent lookup logic — same fallback
	// chain as x/l: exact path → split normalization → global search.
	// We discard the password bytes immediately.
	fmt.Printf("Looking up entry for '%s'...\n", archivePath)
	password, entryPath, err := GetPasswordForArchive(kp, cfg.General.DefaultGroup, archivePath)
	if err != nil {
		if IsPasswordNotFound(err) {
			return fmt.Errorf("no KeePassXC entry found for '%s' in group '%s'", archivePath, cfg.General.DefaultGroup)
		}
		return fmt.Errorf("failed to look up entry: %w", err)
	}
	// Zero out the password — we only needed it to verify the entry exists
	for i := range password {
		password[i] = 0
	}

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Printf("This will delete the KeePassXC entry: %s\n", entryPath)
		fmt.Print("Are you sure? [y/N]: ")
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := kp.DeleteEntry(entryPath); err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	fmt.Printf("Entry '%s' deleted from KeePassXC.\n", entryPath)
	return nil
}
