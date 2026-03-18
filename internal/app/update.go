package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "u <archive_path> [files...]",
	Short:   "Update files to archive",
	Long:    `Updates files in an existing encrypted archive. Only newer files are added.`,
	Args:    cobra.MinimumNArgs(2),
	RunE:    runUpdate,
	GroupID: "actions",
}

func init() {
	updateCmd.Flags().Bool("fast", false, "Fastest compression (-mx=1)")
	updateCmd.Flags().Bool("best", false, "Best compression (-mx=9)")
	updateCmd.MarkFlagsMutuallyExclusive("fast", "best")

	updateCmd.FParseErrWhitelist.UnknownFlags = true
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archiveName := args[0]

	if filepath.Ext(archiveName) == "" {
		archiveName += ".7z"
	}

	var files, extraFlags []string
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			if _, err := os.Stat(arg); err == nil {
				return fmt.Errorf("security policy: archiving files starting with '-' is not supported to prevent parameter injection: %s", arg)
			}
			extraFlags = append(extraFlags, arg)
		} else {
			files = append(files, arg)
		}
	}

	return withKeePassArchive(archiveName, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Updating archive '%s'...\n", archiveName)
		sevenZipArgs := []string{"u"}

		sevenZipArgs = append(sevenZipArgs, getCompressionFlags(cmd)...)
		sevenZipArgs = append(sevenZipArgs, archiveName)
		sevenZipArgs = append(sevenZipArgs, files...)
		sevenZipArgs = append(sevenZipArgs, extraFlags...)

		if err := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); err != nil {
			return fmt.Errorf("failed to update archive: %w", err)
		}

		fmt.Println("Archive updated successfully.")
		return nil
	})
}
