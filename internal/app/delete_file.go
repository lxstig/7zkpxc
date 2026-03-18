package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var deleteFileCmd = &cobra.Command{
	Use:     "d <archive_path> <file_names>...",
	Short:   "Delete files from inside an archive",
	Long:    `Deletes specific files or directories from inside an existing encrypted 7z archive.`,
	Args:    cobra.MinimumNArgs(2),
	RunE:    runDeleteFile,
	GroupID: "actions",
}

func init() {
	rootCmd.AddCommand(deleteFileCmd)
}

func runDeleteFile(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]
	filesToDelete := args[1:]

	return withKeePassArchive(archivePath, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Deleting %d file(s) from '%s'...\n", len(filesToDelete), archivePath)

		sevenZipArgs := []string{"d", archivePath}
		sevenZipArgs = append(sevenZipArgs, filesToDelete...)

		if runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); runErr != nil {
			return fmt.Errorf("deletion failed: %w", runErr)
		}

		fmt.Println("Success! File(s) deleted from archive.")
		return nil
	})
}
