package app

import (
	"fmt"
	"strings"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "l <archive_path> [7z_flags...]",
	Short:   "List contents of archive",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runList,
	GroupID: "actions",
}

func init() {
	listCmd.Flags().SetInterspersed(false)
	listCmd.FParseErrWhitelist.UnknownFlags = true
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	// Collect pass-through 7z flags
	var extraFlags []string
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			extraFlags = append(extraFlags, arg)
		}
	}

	return withKeePassArchive(archivePath, false, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		fmt.Printf("Listing '%s'...\n", archivePath)
		sevenZipArgs := []string{"l", archivePath}
		sevenZipArgs = append(sevenZipArgs, extraFlags...)

		if runErr := sevenzip.Run(cfg.SevenZip.BinaryPath, password, sevenZipArgs); runErr != nil {
			return fmt.Errorf("list failed: %w", runErr)
		}

		fmt.Println("\nSuccess! Archive listed.")
		return nil
	})
}
