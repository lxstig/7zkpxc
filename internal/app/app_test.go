package app

import (
	"testing"
)

func TestRootCommand_Exists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd is nil")
	}
	if rootCmd.Use != "7zkpxc" {
		t.Errorf("rootCmd.Use = %q, want %q", rootCmd.Use, "7zkpxc")
	}
}

func TestAddCommand_Exists(t *testing.T) {
	if addCmd == nil {
		t.Fatal("addCmd is nil")
	}
	if addCmd.Use != "a <archive_name> [files...]" {
		t.Errorf("addCmd.Use = %q, want %q", addCmd.Use, "a <archive_name> [files...]")
	}
}

func TestExtractCommand_Exists(t *testing.T) {
	if extractCmd == nil {
		t.Fatal("extractCmd is nil")
	}
	if extractCmd.Use != "x <archive_path>" {
		t.Errorf("extractCmd.Use = %q, want %q", extractCmd.Use, "x <archive_path>")
	}
}

func TestListCommand_Exists(t *testing.T) {
	if listCmd == nil {
		t.Fatal("listCmd is nil")
	}
	if listCmd.Use != "l <archive_path>" {
		t.Errorf("listCmd.Use = %q, want %q", listCmd.Use, "l <archive_path>")
	}
}

func TestInitCommand_Exists(t *testing.T) {
	if initCmd == nil {
		t.Fatal("initCmd is nil")
	}
	if initCmd.Use != "init" {
		t.Errorf("initCmd.Use = %q, want %q", initCmd.Use, "init")
	}
}

func TestAddCommand_Flags(t *testing.T) {
	tests := []struct {
		name     string
		flagName string
	}{
		{"fast flag", "fast"},
		{"best flag", "best"},
		{"volume flag", "volume"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := addCmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("Flag %q not found", tt.flagName)
			}
		})
	}
}

func TestCommandGroups(t *testing.T) {
	// Verify commands have correct GroupIDs
	if addCmd.GroupID != "actions" {
		t.Errorf("addCmd.GroupID = %q, want %q", addCmd.GroupID, "actions")
	}
	if extractCmd.GroupID != "actions" {
		t.Errorf("extractCmd.GroupID = %q, want %q", extractCmd.GroupID, "actions")
	}
	if listCmd.GroupID != "actions" {
		t.Errorf("listCmd.GroupID = %q, want %q", listCmd.GroupID, "actions")
	}
	if initCmd.GroupID != "setup" {
		t.Errorf("initCmd.GroupID = %q, want %q", initCmd.GroupID, "setup")
	}
}

func TestExtractCommand_Flags(t *testing.T) {
	flag := extractCmd.Flags().Lookup("output")
	if flag == nil {
		t.Fatal("Flag 'output' not found on extract command")
	}
	if flag.Shorthand != "o" {
		t.Errorf("Flag 'output' shorthand = %q, want %q", flag.Shorthand, "o")
	}
	if flag.DefValue != "" {
		t.Errorf("Flag 'output' default = %q, want empty string", flag.DefValue)
	}
}

func TestRmCommand_Exists(t *testing.T) {
	if rmCmd == nil {
		t.Fatal("rmCmd is nil")
	}
	if rmCmd.Use != "rm <archive_path>" {
		t.Errorf("rmCmd.Use = %q, want %q", rmCmd.Use, "rm <archive_path>")
	}
}

func TestRmCommand_GroupID(t *testing.T) {
	if rmCmd.GroupID != "actions" {
		t.Errorf("rmCmd.GroupID = %q, want %q", rmCmd.GroupID, "actions")
	}
}

func TestRmCommand_Flags(t *testing.T) {
	flag := rmCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("Flag 'force' not found on rm command")
	}
	if flag.Shorthand != "f" {
		t.Errorf("Flag 'force' shorthand = %q, want %q", flag.Shorthand, "f")
	}
}

func TestMvCommand_Exists(t *testing.T) {
	if mvCmd == nil {
		t.Fatal("mvCmd is nil")
	}
	if mvCmd.Use != "mv <old_archive_path> <new_archive_path>" {
		t.Errorf("mvCmd.Use = %q, want %q", mvCmd.Use, "mv <old_archive_path> <new_archive_path>")
	}
	if mvCmd.GroupID != "actions" {
		t.Errorf("mvCmd.GroupID = %q, want %q", mvCmd.GroupID, "actions")
	}
}

func TestMvCommand_NoAliases(t *testing.T) {
	if len(mvCmd.Aliases) != 0 {
		t.Errorf("mvCmd.Aliases = %v, want empty (no aliases allowed)", mvCmd.Aliases)
	}
}

func TestVersionCommand_Exists(t *testing.T) {
	if versionCmd == nil {
		t.Fatal("versionCmd is nil")
	}
	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %q, want %q", versionCmd.Use, "version")
	}
}

func TestSetVersionInfo(t *testing.T) {
	// Save original values
	origVersion := appVersion
	origCommit := appCommit
	origDate := appDate
	defer func() {
		appVersion = origVersion
		appCommit = origCommit
		appDate = origDate
	}()

	SetVersionInfo("1.2.3", "abc123", "2025-01-01")

	if appVersion != "1.2.3" {
		t.Errorf("appVersion = %q, want %q", appVersion, "1.2.3")
	}
	if appCommit != "abc123" {
		t.Errorf("appCommit = %q, want %q", appCommit, "abc123")
	}
	if appDate != "2025-01-01" {
		t.Errorf("appDate = %q, want %q", appDate, "2025-01-01")
	}
}

func TestSubcommands_Registered(t *testing.T) {
	// Execute registers groups and commands
	// We need to check if commands are children of root

	found := map[string]bool{
		"init":    false,
		"a":       false,
		"x":       false,
		"l":       false,
		"d":       false,
		"rm":      false,
		"t":       false,
		"e":       false,
		"rn":      false,
		"u":       false,
		"mv":      false,
		"version": false,
	}

	for _, cmd := range rootCmd.Commands() {
		if _, ok := found[cmd.Name()]; ok {
			found[cmd.Name()] = true
		}
	}

	for name, registered := range found {
		if !registered {
			t.Errorf("Command %q not registered as subcommand of root", name)
		}
	}
}
