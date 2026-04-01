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
	Args:    cobra.MinimumNArgs(1),
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

	// Pre-flight check: ensure input files exist before prompting for KeePassXC password.
	// This prevents frustrating UX where the user unlocks their KDBX vault only to find
	// out they mistyped a filename.
	for _, file := range files {
		if strings.ContainsAny(file, "*?") {
			continue
		}
		if _, err := os.Stat(file); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("cannot update '%s': no such file or directory", file)
			}
			return fmt.Errorf("cannot access '%s': %w", file, err)
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
