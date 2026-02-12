package app

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

// Commands that don't need external dependencies.
var skipDependencyCheck = map[string]bool{
	"init":       true,
	"version":    true,
	"completion": true,
	"help":       true,
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "7zkpxc",
	Short: "A secure wrapper for 7-Zip integrated with KeePassXC",
	Long: `7zkpxc is a CLI tool that wraps 7-Zip to provide seamless encryption
using passwords generated and stored in KeePassXC.

It avoids command-line password leakage by piping strict secrets to 7-Zip.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	rootCmd.PersistentPreRunE = checkDependencies
	setupCustomHelp(rootCmd)

	// Define Groups
	rootCmd.AddGroup(&cobra.Group{ID: "setup", Title: "Setup"})
	rootCmd.AddGroup(&cobra.Group{ID: "actions", Title: "Actions"})

	return rootCmd.Execute()
}

func init() {
	// Global flags can be defined here
}

// checkDependencies verifies that required external tools are available
// in PATH before running any action command.
func checkDependencies(cmd *cobra.Command, args []string) error {
	name := cmd.Name()

	// Walk up to find the actual subcommand name (handles nested commands)
	if cmd.Parent() != nil && cmd.Parent() != rootCmd {
		name = cmd.Parent().Name()
	}

	if skipDependencyCheck[name] {
		return nil
	}

	// Also skip for root command itself (no subcommand specified)
	if cmd == rootCmd {
		return nil
	}

	var missing []string

	if _, err := exec.LookPath("7z"); err != nil {
		missing = append(missing, "7z")
	}

	if _, err := exec.LookPath("keepassxc-cli"); err != nil {
		missing = append(missing, "keepassxc-cli")
	}

	if len(missing) == 0 {
		return nil
	}

	msg := fmt.Sprintf("missing required dependencies: %s\n\nInstall them first:", joinWords(missing))

	for _, dep := range missing {
		switch dep {
		case "7z":
			msg += "\n  7z             → https://7-zip.org"
			msg += "\n                   Arch: pacman -S 7zip"
			msg += "\n                   Debian/Ubuntu: apt install p7zip-full"
		case "keepassxc-cli":
			msg += "\n  keepassxc-cli  → https://keepassxc.org"
			msg += "\n                   Arch: pacman -S keepassxc"
			msg += "\n                   Debian/Ubuntu: apt install keepassxc"
		}
	}

	return fmt.Errorf("%s", msg)
}

func joinWords(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	default:
		result := words[0]
		for i := 1; i < len(words)-1; i++ {
			result += ", " + words[i]
		}
		result += " and " + words[len(words)-1]
		return result
	}
}
