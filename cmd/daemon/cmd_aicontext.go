package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	aicontextJSON    bool
	aicontextCompact bool
)

var aicontextCmd = &cobra.Command{
	Use:   "aicontext",
	Short: "Generate AI context documentation",
	Long: "Generate structured documentation about daemon for use by AI tools.\n" +
		"\n" +
		"Outputs a markdown (or JSON) reference of all commands, flags, and usage\n" +
		"patterns that AI assistants can consume as context.\n" +
		"\n" +
		"Examples:\n" +
		"  daemon aicontext             # Markdown output (default)\n" +
		"  daemon aicontext --json      # Structured JSON\n" +
		"  daemon aicontext --compact   # Shorter output",
	RunE: runAIContext,
}

func init() {
	rootCmd.AddCommand(aicontextCmd)

	aicontextCmd.Flags().BoolVar(&aicontextJSON, "json", false, "Output in JSON format")
	aicontextCmd.Flags().BoolVar(&aicontextCompact, "compact", false, "Shorter output")
}

// aiCommandInfo represents a command for JSON output
type aiCommandInfo struct {
	Name        string          `json:"name"`
	Usage       string          `json:"usage"`
	Description string          `json:"description"`
	Flags       []aiFlagInfo    `json:"flags,omitempty"`
	Subcommands []aiCommandInfo `json:"subcommands,omitempty"`
}

// aiFlagInfo represents a flag for JSON output
type aiFlagInfo struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default"`
	Description string `json:"description"`
	Global      bool   `json:"global,omitempty"`
}

// aiContextDoc represents the full AI context for JSON output
type aiContextDoc struct {
	Tool        string          `json:"tool"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	GlobalFlags []aiFlagInfo    `json:"global_flags,omitempty"`
	Commands    []aiCommandInfo `json:"commands"`
}

func runAIContext(cmd *cobra.Command, _ []string) error {
	if aicontextJSON {
		return printAIContextJSON(cmd)
	}

	if aicontextCompact {
		return printAIContextCompact(cmd)
	}

	return printAIContextMarkdown(cmd)
}

func printAIContextMarkdown(cmd *cobra.Command) error {
	var b strings.Builder

	b.WriteString("# daemon - AI Context\n\n")
	b.WriteString("## Overview\n\n")
	b.WriteString("daemon is a CLI application\n\n")

	// Global flags
	var globalFlags []FlagDetail
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}

		globalFlags = append(globalFlags, FlagDetail{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		})
	})

	if len(globalFlags) > 0 {
		b.WriteString("## Global Flags\n\n")
		b.WriteString("These flags apply to all commands:\n\n")

		for _, f := range globalFlags {
			b.WriteString(fmt.Sprintf("- `--%s`", f.Name))

			if f.Shorthand != "" {
				b.WriteString(fmt.Sprintf(", `-%s`", f.Shorthand))
			}

			b.WriteString(fmt.Sprintf(" - %s", f.Description))

			if f.Default != "" && f.Default != "false" {
				b.WriteString(fmt.Sprintf(" (default: %s)", f.Default))
			}

			b.WriteString("\n")
		}

		b.WriteString("\n")
	}

	// Commands
	b.WriteString("## Commands\n\n")
	aiWriteCommandMarkdown(&b, rootCmd.Commands(), "")

	// Categories — customize for your app
	// TODO: customize for your app
	categories := map[string][]string{
		"Core":    {"version", "help"},
		"Tooling": {"cmdtree", "aicontext"},
	}

	b.WriteString("## Command Categories\n\n")
	for cat, cmds := range categories {
		_, _ = fmt.Fprintf(&b, "### %s\n\n", cat)
		for _, name := range cmds {
			_, _ = fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}

	// Structure — customize for your app
	// TODO: customize for your app
	b.WriteString("## Project Structure\n\n")
	b.WriteString("```\n")
	structure := []string{
		"cmd/daemon/  # CLI entry point and commands",
		"internal/          # Private application code",
		"pkg/               # Public reusable libraries",
	}
	for _, s := range structure {
		_, _ = fmt.Fprintf(&b, "%s\n", s)
	}
	b.WriteString("```\n")

	_, _ = fmt.Fprint(cmd.OutOrStdout(), b.String())

	return nil
}

func printAIContextCompact(cmd *cobra.Command) error {
	var b strings.Builder

	b.WriteString("# daemon - daemon is a CLI application\n\n")

	// Global flags
	var globalParts []string
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}

		globalParts = append(globalParts, fmt.Sprintf("`--%s` %s", f.Name, f.Usage))
	})

	if len(globalParts) > 0 {
		b.WriteString("**Global:** ")
		b.WriteString(strings.Join(globalParts, ", "))
		b.WriteString("\n\n")
	}

	// Commands
	for _, c := range rootCmd.Commands() {
		if c.Hidden {
			continue
		}

		aiWriteCompactCommand(&b, c, "")
	}

	_, _ = fmt.Fprint(cmd.OutOrStdout(), b.String())

	return nil
}

func printAIContextJSON(cmd *cobra.Command) error {
	doc := aiContextDoc{
		Tool:        "daemon",
		Version:     "dev",
		Description: "daemon is a CLI application",
	}

	// Global flags
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}

		doc.GlobalFlags = append(doc.GlobalFlags, aiFlagInfo{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
			Global:      true,
		})
	})

	// Commands
	for _, c := range rootCmd.Commands() {
		if c.Hidden {
			continue
		}

		doc.Commands = append(doc.Commands, aiBuildCommandInfo(c))
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")

	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("json encode: %w", err)
	}

	return nil
}

func aiBuildCommandInfo(cmd *cobra.Command) aiCommandInfo {
	info := aiCommandInfo{
		Name:        cmd.Name(),
		Usage:       cmd.UseLine(),
		Description: cmd.Short,
	}

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}

		info.Flags = append(info.Flags, aiFlagInfo{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
			Description: f.Usage,
		})
	})

	for _, sub := range cmd.Commands() {
		if sub.Hidden {
			continue
		}

		info.Subcommands = append(info.Subcommands, aiBuildCommandInfo(sub))
	}

	return info
}

func aiWriteCommandMarkdown(b *strings.Builder, commands []*cobra.Command, prefix string) {
	for _, c := range commands {
		if c.Hidden {
			continue
		}

		heading := "###"
		if prefix != "" {
			heading = "####"
		}

		name := c.Name()
		if prefix != "" {
			name = prefix + " " + name
		}

		_, _ = fmt.Fprintf(b, "%s %s\n\n", heading, name)
		_, _ = fmt.Fprintf(b, "Usage: `%s`\n\n", c.UseLine())
		_, _ = fmt.Fprintf(b, "%s\n\n", c.Short)

		// Flags
		hasFlags := false

		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "help" {
				return
			}

			if !hasFlags {
				b.WriteString("Flags:\n")

				hasFlags = true
			}

			_, _ = fmt.Fprintf(b, "- `--%s`", f.Name)

			if f.Shorthand != "" {
				_, _ = fmt.Fprintf(b, ", `-%s`", f.Shorthand)
			}

			_, _ = fmt.Fprintf(b, " - %s", f.Usage)

			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
				_, _ = fmt.Fprintf(b, " (default: %s)", f.DefValue)
			}

			b.WriteString("\n")
		})

		if hasFlags {
			b.WriteString("\n")
		}

		// Subcommands
		if len(c.Commands()) > 0 {
			aiWriteCommandMarkdown(b, c.Commands(), c.Name())
		}
	}
}

func aiWriteCompactCommand(b *strings.Builder, cmd *cobra.Command, prefix string) {
	name := cmd.Name()
	if prefix != "" {
		name = prefix + " " + name
	}

	_, _ = fmt.Fprintf(b, "- `daemon %s` - %s", name, cmd.Short)

	// Inline flags
	var flagParts []string

	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}

		flagParts = append(flagParts, fmt.Sprintf("`--%s`", f.Name))
	})

	if len(flagParts) > 0 {
		_, _ = fmt.Fprintf(b, " [%s]", strings.Join(flagParts, ", "))
	}

	b.WriteString("\n")

	for _, sub := range cmd.Commands() {
		if sub.Hidden {
			continue
		}

		aiWriteCompactCommand(b, sub, name)
	}
}
