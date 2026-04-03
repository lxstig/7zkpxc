package app

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestSortedCommands_OrderPreserved(t *testing.T) {
	// Create mock commands
	cmds := []*cobra.Command{
		{Use: "version"},
		{Use: "a"},
		{Use: "init"},
		{Use: "x"},
		{Use: "l"},
	}

	sorted := sortedCommands(cmds)

	expectedOrder := []string{"init", "a", "l", "x", "version"}
	for i, cmd := range sorted {
		if cmd.Name() != expectedOrder[i] {
			t.Errorf("position %d: got %q, want %q", i, cmd.Name(), expectedOrder[i])
		}
	}
}

func TestSortedCommands_UnknownCommandsLast(t *testing.T) {
	cmds := []*cobra.Command{
		{Use: "unknown"},
		{Use: "init"},
	}

	sorted := sortedCommands(cmds)
	if sorted[0].Name() != "init" {
		t.Errorf("init should come before unknown, got %q first", sorted[0].Name())
	}
	if sorted[1].Name() != "unknown" {
		t.Errorf("unknown should come after init, got %q second", sorted[1].Name())
	}
}

func TestHasAnnotation(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("fast", false, "fast mode")
	if err := cmd.Flags().SetAnnotation("fast", "compression", []string{"true"}); err != nil {
		t.Fatal(err)
	}

	if !hasAnnotation(cmd, "compression") {
		t.Error("hasAnnotation should return true for 'compression'")
	}
	if hasAnnotation(cmd, "nonexistent") {
		t.Error("hasAnnotation should return false for 'nonexistent'")
	}
}

func TestHasOtherFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("help", false, "help") // No annotation

	if !hasOtherFlags(cmd) {
		t.Error("hasOtherFlags should return true when flags without annotations exist")
	}

	// Command with only annotated flags
	cmd2 := &cobra.Command{Use: "test2"}
	cmd2.Flags().Bool("fast", false, "fast")
	if err := cmd2.Flags().SetAnnotation("fast", "compression", []string{"true"}); err != nil {
		t.Fatal(err)
	}

	if hasOtherFlags(cmd2) {
		t.Error("hasOtherFlags should return false when all flags have annotations")
	}
}

func TestFormatFlag(t *testing.T) {
	// Boolean flag with shorthand
	f := &pflag.Flag{
		Name:      "force",
		Shorthand: "f",
		Usage:     "Force operation",
		Value:     newBoolValue(),
	}

	result := formatFlag(f)
	if !strings.Contains(result, "-f") {
		t.Errorf("formatFlag should contain shorthand -f, got %q", result)
	}
	if !strings.Contains(result, "--force") {
		t.Errorf("formatFlag should contain --force, got %q", result)
	}
	if !strings.Contains(result, "Force operation") {
		t.Errorf("formatFlag should contain usage text, got %q", result)
	}
}

// Helper to create a bool pflag.Value
type boolVal bool

func (b *boolVal) String() string     { return "false" }
func (b *boolVal) Set(s string) error { return nil }
func (b *boolVal) Type() string       { return "bool" }

func newBoolValue() *boolVal {
	v := boolVal(false)
	return &v
}

// Helper to create a string pflag.Value
type strVal string

func (s *strVal) String() string     { return string(*s) }
func (s *strVal) Set(v string) error { *s = strVal(v); return nil }
func (s *strVal) Type() string       { return "string" }

func newStrValue() *strVal {
	v := strVal("")
	return &v
}

// -------------------------------------------------------------------
// formatFlag — no shorthand branch (77.8% → 100%)
// -------------------------------------------------------------------

func TestFormatFlag_NoShorthand(t *testing.T) {
	f := &pflag.Flag{
		Name:  "no-verify",
		Usage: "Skip verification",
		Value: newBoolValue(),
	}

	result := formatFlag(f)
	if !strings.Contains(result, "--no-verify") {
		t.Errorf("formatFlag should contain --no-verify, got %q", result)
	}
	// Should NOT contain "-X, " shorthand prefix
	if strings.Contains(result, "  -") && !strings.Contains(result, "      --") {
		t.Errorf("formatFlag with no shorthand should use '      --' format, got %q", result)
	}
	if !strings.Contains(result, "Skip verification") {
		t.Errorf("formatFlag should contain usage text, got %q", result)
	}
}

func TestFormatFlag_NonBoolType(t *testing.T) {
	f := &pflag.Flag{
		Name:      "output",
		Shorthand: "o",
		Usage:     "Output directory",
		Value:     newStrValue(),
	}

	result := formatFlag(f)
	if !strings.Contains(result, "string") {
		t.Errorf("formatFlag for non-bool should show type 'string', got %q", result)
	}
}

func TestFormatFlag_LongName(t *testing.T) {
	// Test the padding adjustment when flag name is very long
	f := &pflag.Flag{
		Name:  "this-is-a-very-long-flag-name",
		Usage: "Long flag",
		Value: newBoolValue(),
	}

	result := formatFlag(f)
	if !strings.Contains(result, "--this-is-a-very-long-flag-name") {
		t.Errorf("formatFlag should contain the long flag name, got %q", result)
	}
	if !strings.Contains(result, "Long flag") {
		t.Errorf("formatFlag should contain usage text even with long name, got %q", result)
	}
}

// -------------------------------------------------------------------
// flagUsages (0% → 100%)
// -------------------------------------------------------------------

func TestFlagUsages(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("fast", false, "Fast mode")
	cmd.Flags().Bool("best", false, "Best mode")
	cmd.Flags().Bool("help", false, "Show help")

	if err := cmd.Flags().SetAnnotation("fast", "compression", []string{"true"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().SetAnnotation("best", "compression", []string{"true"}); err != nil {
		t.Fatal(err)
	}

	result := flagUsages(cmd, "compression")
	if !strings.Contains(result, "--fast") {
		t.Errorf("flagUsages should contain --fast, got %q", result)
	}
	if !strings.Contains(result, "--best") {
		t.Errorf("flagUsages should contain --best, got %q", result)
	}
	// --help has no annotation, should NOT appear
	if strings.Contains(result, "--help") {
		t.Errorf("flagUsages should not contain --help (no annotation), got %q", result)
	}
}

func TestFlagUsages_NoMatches(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("help", false, "Show help")

	result := flagUsages(cmd, "nonexistent")
	if result != "" {
		t.Errorf("flagUsages for nonexistent annotation should be empty, got %q", result)
	}
}

// -------------------------------------------------------------------
// otherFlagUsages (0% → 100%)
// -------------------------------------------------------------------

func TestOtherFlagUsages(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("help", false, "Show help")                 // no annotation
	cmd.Flags().Bool("no-verify", false, "Skip password verify") // no annotation
	cmd.Flags().Bool("fast", false, "Fast mode")                 // will get annotation

	if err := cmd.Flags().SetAnnotation("fast", "compression", []string{"true"}); err != nil {
		t.Fatal(err)
	}

	result := otherFlagUsages(cmd)
	if !strings.Contains(result, "--help") {
		t.Errorf("otherFlagUsages should contain --help, got %q", result)
	}
	if !strings.Contains(result, "--no-verify") {
		t.Errorf("otherFlagUsages should contain --no-verify, got %q", result)
	}
	// --fast has annotation, should NOT appear
	if strings.Contains(result, "--fast") {
		t.Errorf("otherFlagUsages should not contain --fast (has annotation), got %q", result)
	}
}

func TestOtherFlagUsages_AllAnnotated(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool("fast", false, "Fast mode")
	if err := cmd.Flags().SetAnnotation("fast", "compression", []string{"true"}); err != nil {
		t.Fatal(err)
	}

	result := otherFlagUsages(cmd)
	if result != "" {
		t.Errorf("otherFlagUsages should be empty when all flags are annotated, got %q", result)
	}
}

// -------------------------------------------------------------------
// setupCustomHelp (0% → 100%)
// -------------------------------------------------------------------

func TestSetupCustomHelp(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	setupCustomHelp(cmd)

	// Verify the help template was set (should contain our custom markers)
	tmpl := cmd.HelpTemplate()
	if !strings.Contains(tmpl, "sortedCommands") {
		t.Error("setupCustomHelp should set template containing 'sortedCommands'")
	}
	if !strings.Contains(tmpl, "hasAnnotation") {
		t.Error("setupCustomHelp should set template containing 'hasAnnotation'")
	}
	if !strings.Contains(tmpl, "flagUsages") {
		t.Error("setupCustomHelp should set template containing 'flagUsages'")
	}
}

// -------------------------------------------------------------------
// sortedCommands — duplicate priority (85.7% → 100%)
// -------------------------------------------------------------------

func TestSortedCommands_SamePriority_AlphabeticalFallback(t *testing.T) {
	// Two unknown commands (both priority 100) should sort alphabetically
	cmds := []*cobra.Command{
		{Use: "zebra"},
		{Use: "alpha"},
	}

	sorted := sortedCommands(cmds)
	if sorted[0].Name() != "alpha" {
		t.Errorf("expected alpha first, got %q", sorted[0].Name())
	}
	if sorted[1].Name() != "zebra" {
		t.Errorf("expected zebra second, got %q", sorted[1].Name())
	}
}
