package app

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestJoinWords_Empty(t *testing.T) {
	if got := joinWords(nil); got != "" {
		t.Errorf("joinWords(nil) = %q, want \"\"", got)
	}
}

func TestJoinWords_Single(t *testing.T) {
	if got := joinWords([]string{"7z"}); got != "7z" {
		t.Errorf("joinWords([7z]) = %q, want \"7z\"", got)
	}
}

func TestJoinWords_Two(t *testing.T) {
	if got := joinWords([]string{"7z", "keepassxc-cli"}); got != "7z and keepassxc-cli" {
		t.Errorf("joinWords([7z, keepassxc-cli]) = %q, want \"7z and keepassxc-cli\"", got)
	}
}

func TestJoinWords_Three(t *testing.T) {
	got := joinWords([]string{"a", "b", "c"})
	if got != "a, b and c" {
		t.Errorf("joinWords([a, b, c]) = %q, want \"a, b and c\"", got)
	}
}

func TestCheckDependencies_SkipInit(t *testing.T) {
	// init command should skip dependency checks
	err := checkDependencies(initCmd, nil)
	if err != nil {
		t.Errorf("checkDependencies should skip init, got: %v", err)
	}
}

func TestCheckDependencies_SkipVersion(t *testing.T) {
	err := checkDependencies(versionCmd, nil)
	if err != nil {
		t.Errorf("checkDependencies should skip version, got: %v", err)
	}
}

func TestCheckDependencies_SkipRoot(t *testing.T) {
	err := checkDependencies(rootCmd, nil)
	if err != nil {
		t.Errorf("checkDependencies should skip rootCmd, got: %v", err)
	}
}

func TestCheckDependencies_NestedCommand(t *testing.T) {
	// Create a nested command structure: rootCmd > parent > child
	parent := &cobra.Command{Use: "init"} // "init" is in skipDependencyCheck
	child := &cobra.Command{Use: "sub"}
	parent.AddCommand(child)
	rootCmd.AddCommand(parent)
	defer rootCmd.RemoveCommand(parent)

	// The nested command should look up its parent's name ("init") and skip
	err := checkDependencies(child, nil)
	if err != nil {
		t.Errorf("checkDependencies should skip nested command under 'init', got: %v", err)
	}
}

func TestCheckDependencies_ActionCommand(t *testing.T) {
	// Create a command NOT in skipDependencyCheck
	actionCmd := &cobra.Command{Use: "custom-action"}
	rootCmd.AddCommand(actionCmd)
	defer rootCmd.RemoveCommand(actionCmd)

	// This will either succeed (both deps found) or fail (deps missing)
	// Either way, it exercises the dependency resolution code path
	err := checkDependencies(actionCmd, nil)
	if err != nil {
		// On CI or machines without deps, this is expected — verify it's a proper error
		if !strings.Contains(err.Error(), "missing required dependencies") {
			t.Errorf("expected 'missing required dependencies' error, got: %v", err)
		}
	}
	// If no error, deps are installed — that's fine too
}

func TestCheckDependencies_SkipCompletion(t *testing.T) {
	// "completion" is in skipDependencyCheck
	completionCmd := &cobra.Command{Use: "completion"}
	rootCmd.AddCommand(completionCmd)
	defer rootCmd.RemoveCommand(completionCmd)

	err := checkDependencies(completionCmd, nil)
	if err != nil {
		t.Errorf("checkDependencies should skip completion, got: %v", err)
	}
}
