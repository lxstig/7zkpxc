package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <archive_path>",
	Short:   "Delete KeePassXC entry and the archive file",
	Long:    `Removes the password entry stored in KeePassXC for the given archive, and prompts to delete the local archive file.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runRemove,
	GroupID: "actions",
}

func init() {
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	return withKeePassArchive(archivePath, true, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("This will delete the KeePassXC entry: %s\n", entryPath)
			fmt.Print("Are you sure? [y/N]: ")

			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("failed to read confirmation")
			}
			confirm := strings.TrimSpace(scanner.Text())

			if confirm != "y" && confirm != "Y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := kp.DeleteEntry(entryPath); err != nil {
			return fmt.Errorf("failed to delete entry: %w", err)
		}

		fmt.Printf("Entry '%s' deleted from KeePassXC.\n", entryPath)

		// Also delete the local archive file
		if err := os.Remove(archivePath); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("Warning: failed to delete local archive file '%s': %v\n", archivePath, err)
			}
		} else {
			fmt.Printf("Local archive file '%s' deleted.\n", archivePath)
		}

		return nil
	})
}
