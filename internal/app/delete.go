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
		return nil
	})
}
