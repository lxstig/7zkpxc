package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var renameFileCmd = &cobra.Command{
	Use:   "rn <archive_path> <old_name> <new_name> [old_name_2 new_name_2...]",
	Short: "Rename files in archive",
	Long:  `Renames files or directories inside an encrypted 7z archive without extracting them. Requires pairs of old names and new names.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 3 {
			return fmt.Errorf("requires at least 3 arg(s), only received %d", len(args))
		}
		if (len(args)-1)%2 != 0 {
			return fmt.Errorf("requires an even number of name arguments (old_name new_name pairs)")
		}
		return nil
	},
	RunE:    runRenameFile,
	GroupID: "actions",
}

func init() {
	rootCmd.AddCommand(renameFileCmd)
}

func runRenameFile(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]
	renamePairs := args[1:]

	return withKeePassArchive(archivePath, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Renaming %d file(s) inside '%s'...\n", len(renamePairs)/2, archivePath)

		sevenZipArgs := []string{"rn", archivePath}
		sevenZipArgs = append(sevenZipArgs, renamePairs...)

		if runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); runErr != nil {
			return fmt.Errorf("rename failed: %w", runErr)
		}

		fmt.Println("Success! File(s) renamed inside archive.")
		return nil
	})
}
