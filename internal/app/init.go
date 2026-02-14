package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
	"github.com/lxstig/7zkpxc/internal/config"
	"github.com/spf13/cobra"
)

var errInitCancelled = errors.New("setup cancelled")

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	Long: `Interactive setup wizard to configure KeePassXC database location
and other preferences for 7zkpxc.

Supports Tab completion for file and directory paths.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return runInit()
	},
	GroupID:               "setup",
	DisableFlagsInUseLine: true,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// fileCompleter implements readline.AutoCompleter for filesystem paths.
type fileCompleter struct {
	filter func(path string, isDir bool) bool
}

func (fc *fileCompleter) Do(line []rune, pos int) ([][]rune, int) {
	input := string(line[:pos])

	// Empty input → list current directory
	if input == "" {
		return fc.listDir(".", ""), 0
	}

	// Expand ~ at the beginning
	expanded := input
	homePrefix := ""
	if strings.HasPrefix(expanded, "~/") || expanded == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			homePrefix = "~/"
			if expanded == "~" {
				expanded = home
			} else {
				expanded = filepath.Join(home, expanded[2:])
			}
		}
	}

	// Determine directory to list and prefix already typed
	dir := expanded
	prefix := ""
	if !isDir(expanded) {
		dir = filepath.Dir(expanded)
		prefix = filepath.Base(expanded)
	}

	_ = homePrefix
	return fc.listDir(dir, prefix), len(prefix)
}

func (fc *fileCompleter) listDir(dir, prefix string) [][]rune {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var candidates [][]rune
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless user explicitly typed a dot
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}

		entryPath := filepath.Join(dir, name)
		entryIsDir := entry.IsDir()

		// Apply filter (e.g. only show .kdbx files and directories)
		if fc.filter != nil && !fc.filter(entryPath, entryIsDir) {
			continue
		}

		suffix := name[len(prefix):]
		if entryIsDir {
			suffix += "/"
		} else {
			suffix += " "
		}

		candidates = append(candidates, []rune(suffix))
	}

	return candidates
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// kdbxFilter shows only directories and .kdbx files during tab completion.
func kdbxFilter(path string, isDir bool) bool {
	if isDir {
		return true
	}
	return strings.HasSuffix(strings.ToLower(path), ".kdbx")
}

func runInit() error {
	cfg := &config.Config{
		General: config.GeneralConfig{
			UseKeyring:     true,
			PasswordLength: config.PasswordLengthDefault,
		},
		SevenZip: config.SevenZipConfig{
			DefaultArgs: []string{"-mhe=on", "-mx=9"},
			BinaryPath:  "7z",
		},
	}

	fmt.Println("Welcome to 7zkpxc setup!")
	fmt.Println("========================")
	fmt.Println()

	// --- Step 1: KDBX Path (with tab completion) ---
	kdbxPath, err := promptKdbxPath()
	if err != nil {
		if errors.Is(err, errInitCancelled) {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}
	cfg.General.KdbxPath = kdbxPath

	// --- Step 2: Default Group ---
	group, err := promptGroup()
	if err != nil {
		if errors.Is(err, errInitCancelled) {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}
	cfg.General.DefaultGroup = group

	// --- Step 3: Password Length ---
	length, err := promptPasswordLength()
	if err != nil {
		if errors.Is(err, errInitCancelled) {
			fmt.Println("\nSetup cancelled.")
			return nil
		}
		return err
	}
	cfg.General.PasswordLength = length

	// --- Step 3: Save with Comments ---
	if err := saveConfigWithComments(cfg); err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}

	fmt.Println()
	fmt.Println("Configuration saved to ~/.config/7zkpxc/config.yaml")
	fmt.Printf("  DB Path : %s\n", cfg.General.KdbxPath)
	fmt.Printf("  Group   : %s\n", cfg.General.DefaultGroup)
	fmt.Printf("  Length  : %d\n", cfg.General.PasswordLength)
	return nil
}

func promptKdbxPath() (string, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "Path to your .kdbx database: ",
		AutoComplete: &fileCompleter{filter: kdbxFilter},

		// Disable history file for this one-off prompt
		HistoryFile: "",

		// Let Ctrl+C cancel gracefully
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return "", fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() { _ = rl.Close() }()

	for {
		line, err := rl.Readline()
		if err != nil {
			return "", errInitCancelled
		}

		raw := strings.TrimSpace(line)
		if raw == "" {
			fmt.Println("  Path cannot be empty.")
			continue
		}

		resolved := expandAndResolve(raw)

		info, statErr := os.Stat(resolved)
		if statErr != nil {
			fmt.Printf("  Not found: %s\n", resolved)
			continue
		}

		if info.IsDir() {
			fmt.Printf("  '%s' is a directory, not a file.\n", resolved)
			continue
		}

		if !strings.HasSuffix(strings.ToLower(resolved), ".kdbx") {
			fmt.Printf("  Warning: '%s' doesn't have a .kdbx extension.\n", filepath.Base(resolved))

			confirmRL, cErr := readline.NewEx(&readline.Config{
				Prompt: "  Use anyway? [y/N]: ",
			})
			if cErr == nil {
				answer, _ := confirmRL.Readline()
				_ = confirmRL.Close()
				answer = strings.TrimSpace(answer)
				if answer != "y" && answer != "Y" {
					continue
				}
			}
		}

		return resolved, nil
	}
}

func promptGroup() (string, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:      "Default KeePassXC group for archives [Archives/AutoGenerated]: ",
		HistoryFile: "",
	})
	if err != nil {
		return "", fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() { _ = rl.Close() }()

	line, err := rl.Readline()
	if err != nil {
		return "", errInitCancelled
	}

	group := strings.TrimSpace(line)
	if group == "" {
		group = "Archives/AutoGenerated"
	}
	return group, nil
}

func promptPasswordLength() (int, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:      fmt.Sprintf("Password length (min %d, max %d) [%d]: ", config.PasswordLengthMin, config.PasswordLengthMax, config.PasswordLengthDefault),
		HistoryFile: "",
	})
	if err != nil {
		return 0, fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() { _ = rl.Close() }()

	for {
		line, err := rl.Readline()
		if err != nil {
			return 0, errInitCancelled
		}

		raw := strings.TrimSpace(line)
		if raw == "" {
			return config.PasswordLengthDefault, nil
		}

		var val int
		_, err = fmt.Sscanf(raw, "%d", &val)
		if err != nil {
			fmt.Println("  Please enter a valid number.")
			continue
		}

		if val < config.PasswordLengthMin || val > config.PasswordLengthMax {
			fmt.Printf("  Password length must be between %d and %d.\n", config.PasswordLengthMin, config.PasswordLengthMax)
			continue
		}

		return val, nil
	}
}

func expandAndResolve(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, path[2:])
			}
		}
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func saveConfigWithComments(cfg *config.Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configDir := filepath.Join(home, ".config", "7zkpxc")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	content := fmt.Sprintf(`general:
  kdbx_path: "%s"
  default_group: "%s"
  use_keyring: %t
  # generated password length (min: %d, max: %d)
  password_length: %d
sevenzip:
  binary_path: "%s"
  default_args: [%s]
`,
		cfg.General.KdbxPath,
		cfg.General.DefaultGroup,
		cfg.General.UseKeyring,
		config.PasswordLengthMin,
		config.PasswordLengthMax,
		cfg.General.PasswordLength,
		cfg.SevenZip.BinaryPath,
		formatStringSlice(cfg.SevenZip.DefaultArgs),
	)

	configPath := filepath.Join(configDir, "config.yaml")
	return os.WriteFile(configPath, []byte(content), 0644)
}

func formatStringSlice(s []string) string {
	var quoted []string
	for _, v := range s {
		quoted = append(quoted, fmt.Sprintf("%q", v))
	}
	return strings.Join(quoted, ", ")
}
