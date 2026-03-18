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
