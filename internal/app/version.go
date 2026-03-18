package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

// SetVersionInfo sets the version information from build-time ldflags.
func SetVersionInfo(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("7zkpxc %s\n", appVersion)
		fmt.Printf("  commit:  %s\n", appCommit)
		fmt.Printf("  built:   %s\n", appDate)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

		home, err := os.UserHomeDir()
		if err == nil {
			configPath := filepath.Join(home, ".config", "7zkpxc", "config.yaml")
			if _, statErr := os.Stat(configPath); statErr == nil {
				fmt.Printf("  config:  %s\n", configPath)
			} else {
				fmt.Printf("  config:  (not found)\n")
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
