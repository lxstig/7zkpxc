package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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

// kdbxPainter implements readline.Painter to color .kdbx filenames green
// in the input line. Paint is called on every redraw; it returns the runes
// that will be written to the terminal (ANSI sequences are safe here because
// they never enter the internal buffer — only the display output).
type kdbxPainter struct{}

const (
	ansiGreen = "\033[32m"
	ansiReset = "\033[0m"
)

func (kdbxPainter) Paint(line []rune, _ int) []rune {
	s := string(line)
	// Find the last path segment to check its extension
	last := strings.LastIndex(s, "/")
	seg := s[last+1:]
	if seg == "" || !strings.HasSuffix(strings.ToLower(seg), ".kdbx") {
		return line
	}
	// Color only the final segment (the .kdbx filename)
	colored := s[:last+1] + ansiGreen + seg + ansiReset
	return []rune(colored)
}

// fileCompleter implements readline.AutoCompleter for filesystem paths.
// It only handles SUFFIX appending (chzyer/readline's Do protocol).
// Case correction for mismatched prefixes is handled via the Listener.
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
	expanded := expandTilde(input)

	// Determine directory to list and prefix already typed
	dir := expanded
	prefix := ""
	if !isDir(expanded) {
		dir = filepath.Dir(expanded)
		prefix = filepath.Base(expanded)
	}

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

		// Case-sensitive prefix match only; case correction is handled by the
		// Listener (pathCaseListener) which rewrites the buffer before Do runs.
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}

		entryPath := filepath.Join(dir, name)
		entryIsDir := entry.IsDir()

		// Apply filter (e.g. only show .kdbx files and directories)
		if fc.filter != nil && !fc.filter(entryPath, entryIsDir) {
			continue
		}

		// Return only the suffix (chzyer/readline appends, never deletes).
		// For .kdbx files, wrap the suffix in ANSI green so the completion
		// menu shows them colored. The ANSI codes are stripped from the
		// readline result before the path is used (see stripANSI below).
		suffix := name[len(prefix):]
		if entryIsDir {
			suffix += "/"
		} else {
			if strings.HasSuffix(strings.ToLower(name), ".kdbx") {
				suffix = ansiGreen + suffix + ansiReset
			}
			suffix += " "
		}

		candidates = append(candidates, []rune(suffix))
	}

	return candidates
}

// pathCaseListener returns a readline Listener that intercepts the Tab key.
// When Tab is pressed, it inspects the last path segment the user has typed
// and, if a case-insensitive match exists on disk with different casing,
// rewrites the buffer with the correct filesystem casing BEFORE the
// AutoCompleter's Do() runs (Do sees the corrected line and appends the rest).
func pathCaseListener() func(line []rune, pos int, key rune) ([]rune, int, bool) {
	return func(line []rune, pos int, key rune) ([]rune, int, bool) {
		// Only act on Tab
		if key != '\t' {
			return nil, 0, false
		}

		input := string(line[:pos])
		if input == "" {
			return nil, 0, false
		}

		// Expand ~ if needed
		expanded := expandTilde(input)

		// Only correct when input is a partial path (not yet a directory)
		if isDir(expanded) {
			return nil, 0, false
		}

		dir := filepath.Dir(expanded)
		typedSeg := filepath.Base(expanded)

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, 0, false
		}

		for _, entry := range entries {
			name := entry.Name()
			if strings.EqualFold(name, typedSeg) && name != typedSeg {
				// Found case-insensitive match with different casing;
				// rewrite just the last segment of the typed input.
				corrected := input[:len(input)-len(typedSeg)] + name
				newLine := []rune(corrected)
				if len(line) > pos {
					newLine = append(newLine, line[pos:]...)
				}
				return newLine, len(corrected), true
			}
		}

		// No mismatch found; let readline handle Tab normally
		return nil, 0, false
	}
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// expandTilde replaces a leading "~" or "~/" with the user's home directory.
// If home cannot be determined, the input is returned unchanged.
// Note: this does NOT call filepath.Abs — callers that need absolute paths
// should call expandAndResolve instead.
func expandTilde(s string) string {
	if s == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	} else if strings.HasPrefix(s, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, s[2:])
		}
	}
	return s
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

	// isCancelled handles errInitCancelled uniformly across all prompt steps.
	// Returns true (and prints "Setup cancelled.") when err is errInitCancelled.
	isCancelled := func(err error) bool {
		if errors.Is(err, errInitCancelled) {
			fmt.Println("\nSetup cancelled.")
			return true
		}
		return false
	}

	// --- Step 1: KDBX Path (with tab completion) ---
	kdbxPath, err := promptKdbxPath()
	if err != nil {
		if isCancelled(err) {
			return nil
		}
		return err
	}
	cfg.General.KdbxPath = kdbxPath

	// --- Step 2: Default Group ---
	group, err := promptGroup()
	if err != nil {
		if isCancelled(err) {
			return nil
		}
		return err
	}
	cfg.General.DefaultGroup = group

	// --- Step 3: Password Length ---
	length, err := promptPasswordLength()
	if err != nil {
		if isCancelled(err) {
			return nil
		}
		return err
	}
	cfg.General.PasswordLength = length

	// --- Step 4: 7z Binary ---
	binary, err := promptSevenZipBinary()
	if err != nil {
		if isCancelled(err) {
			return nil
		}
		return err
	}
	cfg.SevenZip.BinaryPath = binary

	// --- Save ---
	if err := saveConfigWithComments(cfg); err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}

	fmt.Println()
	fmt.Println("Configuration saved to ~/.config/7zkpxc/config.yaml")
	fmt.Printf("  DB Path : %s\n", cfg.General.KdbxPath)
	fmt.Printf("  Group   : %s\n", cfg.General.DefaultGroup)
	fmt.Printf("  Length  : %d\n", cfg.General.PasswordLength)
	fmt.Printf("  7z bin  : %s\n", cfg.SevenZip.BinaryPath)
	return nil
}


func promptKdbxPath() (string, error) {
	cfg := &readline.Config{
		Prompt:          "Path to your .kdbx database: ",
		AutoComplete:    &fileCompleter{filter: kdbxFilter},
		HistoryFile:     "",
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		Painter:         kdbxPainter{},
	}
	cfg.SetListener(pathCaseListener())

	rl, err := readline.NewEx(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer func() { _ = rl.Close() }()

	for {
		line, err := rl.Readline()
		if err != nil {
			return "", errInitCancelled
		}

		raw := stripANSI(strings.TrimSpace(line))
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
			if cErr != nil {
				// If we can't display the prompt, don't silently accept — re-prompt.
				continue
			}
			answer, _ := confirmRL.Readline()
			_ = confirmRL.Close()
			answer = strings.TrimSpace(answer)
			if answer != "y" && answer != "Y" {
				continue
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

// detectSevenZipBinary returns the first available 7-Zip binary in PATH.
// Checks in order: 7zz (modern upstream), 7z (legacy p7zip), 7za (standalone).
func detectSevenZipBinary() (string, bool) {
	for _, name := range []string{"7zz", "7z", "7za"} {
		if _, err := exec.LookPath(name); err == nil {
			return name, true
		}
	}
	return "7z", false
}

// promptSevenZipBinary detects available 7z binaries and lets the user confirm or override.
func promptSevenZipBinary() (string, error) {
	detected, found := detectSevenZipBinary()

	var defaultVal string
	if found {
		defaultVal = detected
		fmt.Printf("Detected 7-Zip binary: %s\n", detected)
	} else {
		defaultVal = "7z"
		fmt.Println("Warning: no 7-Zip binary found in PATH (7z, 7zz, 7za).")
		fmt.Println("  You can set the binary name manually below.")
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:      fmt.Sprintf("7-Zip binary name [%s]: ", defaultVal),
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

	input := strings.TrimSpace(line)
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
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

// stripANSI removes ANSI escape sequences (e.g. \033[32m) from s.
// Used to clean readline results that may contain color codes from
// tab-completion candidates (.kdbx files are wrapped in green).
func stripANSI(s string) string {
	out := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] != 'm' {
				i++
			}
			i++ // skip 'm'
			continue
		}
		out = append(out, s[i])
		i++
	}
	return string(out)
}

