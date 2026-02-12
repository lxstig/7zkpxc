package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/lxstig/7zkpxc/internal/sevenzip"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:     "a <archive_name> [files...]",
	Short:   "Add files to archive",
	Args:    cobra.MinimumNArgs(2),
	RunE:    runAdd,
	GroupID: "actions",
}

func init() {
	// Compression Flags
	addCmd.Flags().Bool("fast", false, "Fastest compression (-mx=1)")
	_ = addCmd.Flags().SetAnnotation("fast", "compression", []string{"true"})

	addCmd.Flags().Bool("best", false, "Best compression (-mx=9)")
	_ = addCmd.Flags().SetAnnotation("best", "compression", []string{"true"})

	// Volume Flags
	addCmd.Flags().String("volume", "", "Create volumes (e.g. 100m, 1g)")
	_ = addCmd.Flags().SetAnnotation("volume", "volume", []string{"true"})

	// Allow unknown flags to pass through to 7z (e.g. -sfx, -m0=lzma2)
	addCmd.FParseErrWhitelist.UnknownFlags = true

	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	archiveName := args[0]
	var files []string

	// Ensure archive extension
	if filepath.Ext(archiveName) == "" {
		archiveName += ".7z"
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	// 1. Generate Password
	fmt.Printf("Generating %d-character secure password...\n", cfg.General.PasswordLength)
	password, err := kp.GeneratePassword(cfg.General.PasswordLength)
	if err != nil {
		return fmt.Errorf("failed to generate password: %w", err)
	}

	// 2. Save to KeePassXC
	fmt.Printf("Saving entry to KeePassXC (%s)...\n", cfg.General.KdbxPath)
	absArchivePath, err := filepath.Abs(archiveName)
	if err != nil {
		return fmt.Errorf("failed to resolve archive path: %w", err)
	}

	err = kp.AddEntry(
		cfg.General.DefaultGroup,
		archiveName,
		password,
		absArchivePath,
	)
	if err != nil {
		return fmt.Errorf("failed to add entry to KeePassXC: %w", err)
	}

	// 3. Build 7z arguments
	fmt.Printf("Creating archive '%s'...\n", archiveName)

	sevenZipArgs := []string{"a"}

	// Default args from config
	sevenZipArgs = append(sevenZipArgs, cfg.SevenZip.DefaultArgs...)

	// Handle custom flags
	fast, _ := cmd.Flags().GetBool("fast")
	best, _ := cmd.Flags().GetBool("best")
	volume, _ := cmd.Flags().GetString("volume")

	if fast {
		sevenZipArgs = append(sevenZipArgs, "-mx=1")
	} else if best {
		sevenZipArgs = append(sevenZipArgs, "-mx=9")
	}

	if volume != "" {
		sevenZipArgs = append(sevenZipArgs, "-v"+volume)
	}

	// Separate positional files from pass-through 7z flags (starting with "-")
	// Users can pass raw 7z flags after -- or inline (e.g. `7zkpxc a archive files -sfx`)
	for _, arg := range args[1:] {
		if strings.HasPrefix(arg, "-") {
			sevenZipArgs = append(sevenZipArgs, arg)
		} else {
			files = append(files, arg)
		}
	}

	// Tell 7z to prompt for password (handled securely via PTY)
	sevenZipArgs = append(sevenZipArgs, "-p")

	sevenZipArgs = append(sevenZipArgs, archiveName)
	sevenZipArgs = append(sevenZipArgs, files...)

	return sevenzip.Run(password, sevenZipArgs)
}
