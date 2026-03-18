package app

import (
	"testing"
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
