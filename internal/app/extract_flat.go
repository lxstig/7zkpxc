package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var extractFlatCmd = &cobra.Command{
	Use:     "e <archive_path> [file_names...]",
	Short:   "Extract files from archive (without using directory names)",
	Long:    `Extracts files from the archive directly into the current or specified output directory, ignoring any internal folder structure.`,
	Args:    cobra.MinimumNArgs(1),
	RunE:    runExtractFlat,
	GroupID: "actions",
}

func init() {
	extractFlatCmd.Flags().StringP("output", "o", "", "Output directory for extracted files")
	rootCmd.AddCommand(extractFlatCmd)
}

func runExtractFlat(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]
	filesToExtract := args[1:]

	return withKeePassArchive(archivePath, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Extracting (flat) '%s'...\n", archivePath)
		sevenZipArgs := []string{"e", archivePath}

		if len(filesToExtract) > 0 {
			sevenZipArgs = append(sevenZipArgs, filesToExtract...)
		}

		outputDir, _ := cmd.Flags().GetString("output")
		if outputDir != "" {
			sevenZipArgs = append(sevenZipArgs, "-o"+outputDir)
		}

		if runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); runErr != nil {
			return fmt.Errorf("flat extraction failed: %w", runErr)
		}

		fmt.Println("Success! Archive extracted (flat).")
		return nil
	})
}
