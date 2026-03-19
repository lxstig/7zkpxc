package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:     "x <archive_path> [7z_flags...]",
	Short:   "Extract archive contents",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runExtract,
	GroupID: "actions",
}

func init() {
	extractCmd.Flags().StringP("output", "o", "", "Output directory for extracted files")
	extractCmd.Flags().SetInterspersed(false)
	extractCmd.FParseErrWhitelist.UnknownFlags = true
	rootCmd.AddCommand(extractCmd)
}

func runExtract(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	// Pass all remaining arguments (both flags and specific files) to 7z
	extraArgs := args[1:]

	return withKeePassArchive(archivePath, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Extracting '%s'...\n", archivePath)
		sevenZipArgs := []string{"x", archivePath}

		outputDir, _ := cmd.Flags().GetString("output")
		if outputDir != "" {
			sevenZipArgs = append(sevenZipArgs, "-o"+outputDir)
		}

		sevenZipArgs = append(sevenZipArgs, extraArgs...)

		if runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); runErr != nil {
			return fmt.Errorf("extraction failed: %w", runErr)
		}

		fmt.Println("Success! Archive extracted.")
		return nil
	})
}
