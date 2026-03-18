package app

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/lxstig/7zkpxc/internal/keepass"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <archive_path>",
	Short:   "Delete KeePassXC entry and the archive file",
	Long:    `Removes the password entry stored in KeePassXC for the given archive, and prompts to delete the local archive file. For split archives, all volumes are deleted automatically.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runRemove,
	GroupID: "actions",
}

func init() {
	removeCmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	archivePath := args[0]

	return withKeePassArchive(archivePath, true, func(cfg *config.Config, kp *keepass.Client, password []byte, entryPath string) error {
		force, _ := cmd.Flags().GetBool("force")
		if !force {
			fmt.Printf("This will delete the KeePassXC entry: %s\n", entryPath)
			fmt.Print("Are you sure? [y/N]: ")

			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return fmt.Errorf("failed to read confirmation")
			}
			confirm := strings.TrimSpace(scanner.Text())

			if confirm != "y" && confirm != "Y" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := kp.DeleteEntry(entryPath); err != nil {
			return fmt.Errorf("failed to delete entry: %w", err)
		}

		fmt.Printf("Entry '%s' deleted from KeePassXC.\n", entryPath)

		// Delete archive file(s): handle split archives by removing all volumes
		info := AnalyzeArchive(archivePath)
		if info.IsSplit {
			removeAllSplitVolumes(archivePath)
		} else {
			removeSingleFile(archivePath)
		}

		return nil
	})
}

// removeSingleFile deletes a single file, printing the result.
func removeSingleFile(path string) {
	if err := os.Remove(path); err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Warning: failed to delete '%s': %v\n", path, err)
		}
	} else {
		fmt.Printf("Deleted '%s'.\n", path)
	}
}

// removeAllSplitVolumes removes all volumes of a split archive.
// Example: for "archive.7z.001" it removes archive.7z.001, .002, .003, etc.
func removeAllSplitVolumes(archivePath string) {
	absPath, err := filepath.Abs(archivePath)
	if err != nil {
		removeSingleFile(archivePath)
		return
	}

	info := AnalyzeArchive(absPath)
	dir := filepath.Dir(absPath)

	// Build a glob pattern based on the normalized name.
	// For "archive.7z.001" → normalized = "archive.7z" → glob "archive.7z.[0-9]*"
	// For "archive.part001.rar" → normalized = "archive.rar" → glob "archive.part[0-9]*.rar"
	var pattern string
	switch info.Type {
	case ArchiveSplitStandard:
		pattern = filepath.Join(dir, info.NormalizedName+".[0-9]*")
	case ArchiveSplitRarPart:
		ext := filepath.Ext(info.NormalizedName) // .rar
		base := strings.TrimSuffix(info.NormalizedName, ext)
		pattern = filepath.Join(dir, base+".part[0-9]*"+ext)
	case ArchiveSplitRarOld:
		base := strings.TrimSuffix(info.NormalizedName, filepath.Ext(info.NormalizedName))
		pattern = filepath.Join(dir, base+".r[0-9]*")
	default:
		removeSingleFile(archivePath)
		return
	}

	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		removeSingleFile(archivePath)
		return
	}

	deleted := 0
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			if !os.IsNotExist(err) {
				fmt.Printf("Warning: failed to delete '%s': %v\n", m, err)
			}
		} else {
			deleted++
		}
	}
	fmt.Printf("Deleted %d split volume(s).\n", deleted)
}
