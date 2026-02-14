package app

import (
	"fmt"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:     "d <archive_path>",
	Short:   "Delete KeePassXC entry for an archive",
	Long:    `Removes the password entry stored in KeePassXC for the given archive. This does NOT delete the archive file itself.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runDelete,
	GroupID: "actions",
}

func init() {
	deleteCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	kp := keepass.New(cfg.General.KdbxPath)
	defer kp.Close()

	// Use the fallback strategy to locate the entry
	fmt.Printf("Looking up entry for '%s'...\n", archivePath)
	entryPath, err := resolveEntryPath(kp, cfg.General.DefaultGroup, archivePath)
	if err != nil {
		if IsPasswordNotFound(err) {
			return fmt.Errorf("no KeePassXC entry found for '%s' in group '%s'", archivePath, cfg.General.DefaultGroup)
		}
		return fmt.Errorf("failed to look up entry: %w", err)
	}

	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Printf("This will delete the KeePassXC entry: %s\n", entryPath)
		fmt.Print("Are you sure? [y/N]: ")
		var confirm string
		if _, err := fmt.Scanln(&confirm); err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := kp.DeleteEntry(entryPath); err != nil {
		return fmt.Errorf("failed to delete entry: %w", err)
	}

	fmt.Printf("Entry '%s' deleted from KeePassXC.\n", entryPath)
	return nil
}

// resolveEntryPath attempts to find the full KeePassXC entry path for an archive.
// It uses the same fallback strategy as GetPasswordForArchive but returns
// the resolved entry path instead of the password.
func resolveEntryPath(kp *keepass.Client, entryPathPrefix, archivePath string) (string, error) {
	info := AnalyzeArchive(archivePath)
	tried := make([]string, 0, 3)

	// 1. Try normalized name
	normalizedKey := joinEntry(entryPathPrefix, info.NormalizedName)
	tried = append(tried, normalizedKey)
	if _, err := kp.GetPassword(normalizedKey); err == nil {
		return normalizedKey, nil
	}

	// 2. Try original base name
	if info.NormalizedName != info.OriginalName {
		originalKey := joinEntry(entryPathPrefix, info.OriginalName)
		tried = append(tried, originalKey)
		if _, err := kp.GetPassword(originalKey); err == nil {
			return originalKey, nil
		}
	}

	// 3. For split archives, try base name without extension
	if info.IsSplit {
		baseWithoutExt := info.NormalizedName
		ext := ""
		for i := len(baseWithoutExt) - 1; i >= 0; i-- {
			if baseWithoutExt[i] == '.' {
				ext = baseWithoutExt[i:]
				break
			}
		}
		if ext != "" {
			base := baseWithoutExt[:len(baseWithoutExt)-len(ext)]
			if base != info.NormalizedName && base != info.OriginalName {
				baseKey := joinEntry(entryPathPrefix, base)
				tried = append(tried, baseKey)
				if _, err := kp.GetPassword(baseKey); err == nil {
					return baseKey, nil
				}
			}
		}
	}

	return "", &PasswordNotFoundError{
		ArchiveName: info.OriginalName,
		Tried:       tried,
	}
}
