package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Custom Help Template to group flags and sort commands
const helpTemplate = `{{.Long | trimTrailingWhitespaces}}

Usage:
  {{.UseLine}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range sortedCommands .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}
{{if hasAnnotation . "compression"}}
Compression Flags:
{{flagUsages . "compression"}}{{end}}{{if hasAnnotation . "volume"}}
Volume Flags:
{{flagUsages . "volume"}}{{end}}{{if hasOtherFlags .}}
General Flags:
{{otherFlagUsages .}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// Priority order for commands
var commandOrder = map[string]int{
	"init":    1,
	"a":       2,
	"x":       3,
	"l":       4,
	"d":       5,
	"rename":  6,
	"version": 7,
	// others follow
}

// Helper to sort commands based on priority
func sortedCommands(cmds []*cobra.Command) []*cobra.Command {
	sorted := make([]*cobra.Command, len(cmds))
	copy(sorted, cmds)

	sort.Slice(sorted, func(i, j int) bool {
		c1, c2 := sorted[i], sorted[j]
		p1, ok1 := commandOrder[c1.Name()]
		if !ok1 {
			p1 = 100
		}
		p2, ok2 := commandOrder[c2.Name()]
		if !ok2 {
			p2 = 100
		}

		if p1 != p2 {
			return p1 < p2
		}
		return c1.Name() < c2.Name()
	})

	return sorted
}

// Helper to check if cmd has flags with specific annotation
func hasAnnotation(cmd *cobra.Command, annotation string) bool {
	has := false
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if _, ok := f.Annotations[annotation]; ok {
			has = true
		}
	})
	return has
}

// Helper to check if cmd has "other" flags (no specific annotation)
func hasOtherFlags(cmd *cobra.Command) bool {
	has := false
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if len(f.Annotations) == 0 {
			has = true // At least one flag without annotations (e.g. help)
		}
	})
	return has
}

func formatFlag(f *pflag.Flag) string {
	line := fmt.Sprintf("      --%s", f.Name)
	if f.Shorthand != "" {
		line = fmt.Sprintf("  -%s, --%s", f.Shorthand, f.Name)
	}
	if f.Value.Type() != "bool" {
		line += fmt.Sprintf(" %s", f.Value.Type())
	}
	pad := 30 - len(line)
	if pad < 1 {
		pad = 1
	}
	return line + strings.Repeat(" ", pad) + f.Usage + "\n"
}

// Helper to print usages for specific annotation
func flagUsages(cmd *cobra.Command, annotation string) string {
	var sb strings.Builder
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if _, ok := f.Annotations[annotation]; ok {
			sb.WriteString(formatFlag(f))
		}
	})
	return strings.TrimRight(sb.String(), "\n")
}

// Helper for other flags
func otherFlagUsages(cmd *cobra.Command) string {
	var sb strings.Builder
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if len(f.Annotations) == 0 {
			sb.WriteString(formatFlag(f))
		}
	})
	return strings.TrimRight(sb.String(), "\n")
}

func setupCustomHelp(cmd *cobra.Command) {
	cobra.AddTemplateFunc("hasAnnotation", hasAnnotation)
	cobra.AddTemplateFunc("hasOtherFlags", hasOtherFlags)
	cobra.AddTemplateFunc("flagUsages", flagUsages)
	cobra.AddTemplateFunc("otherFlagUsages", otherFlagUsages)
	cobra.AddTemplateFunc("sortedCommands", sortedCommands)
	cmd.SetHelpTemplate(helpTemplate)
}
