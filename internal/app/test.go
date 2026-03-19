package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:     "t <archive_path> [7z_flags...]",
	Short:   "Test integrity of archive",
	Long:    `Verifies the structural integrity and password correctness of a 7z archive without extracting any files to disk.`,
	Args:    cobra.MinimumNArgs(1),
	RunE:    runTest,
	GroupID: "actions",
}

func init() {
	testCmd.Flags().SetInterspersed(false)
	testCmd.FParseErrWhitelist.UnknownFlags = true
	rootCmd.AddCommand(testCmd)
}

func runTest(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	// Pass all remaining arguments to 7z
	extraArgs := args[1:]

	// readOnly = false -> Allow it to update the location in KeePassXC on a successful test
	return withKeePassArchive(archivePath, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Testing integrity of '%s'...\n", archivePath)

		sevenZipArgs := []string{"t", archivePath}
		sevenZipArgs = append(sevenZipArgs, extraArgs...)

		if runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); runErr != nil {
			return fmt.Errorf("test failed: %w", runErr)
		}

		fmt.Println("Success! Archive integrity verified.")
		return nil
	})
}
