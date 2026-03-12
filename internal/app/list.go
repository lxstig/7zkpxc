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

	return withKeePassArchive(archivePath, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Listing '%s'...\n", archivePath)
		if runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, []string{"l", archivePath}); runErr != nil {
			return fmt.Errorf("list failed: %w", runErr)
		}

		fmt.Println("\nSuccess! Archive listed.")
		return nil
	})
}
