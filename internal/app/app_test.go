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

func TestDeleteCommand_Exists(t *testing.T) {
	if deleteCmd == nil {
		t.Fatal("deleteCmd is nil")
	}
	if deleteCmd.Use != "d <archive_path>" {
		t.Errorf("deleteCmd.Use = %q, want %q", deleteCmd.Use, "d <archive_path>")
	}
}

func TestDeleteCommand_GroupID(t *testing.T) {
	if deleteCmd.GroupID != "actions" {
		t.Errorf("deleteCmd.GroupID = %q, want %q", deleteCmd.GroupID, "actions")
	}
}

func TestDeleteCommand_Flags(t *testing.T) {
	flag := deleteCmd.Flags().Lookup("force")
	if flag == nil {
		t.Fatal("Flag 'force' not found on delete command")
	}
	if flag.Shorthand != "f" {
		t.Errorf("Flag 'force' shorthand = %q, want %q", flag.Shorthand, "f")
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
