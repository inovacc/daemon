package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var jsonOutput bool

// Version information embedded at build time
var (
	// Version is the application version (from git tag or VERSION env)
	Version = "dev"

	// GitHash is the git commit hash
	GitHash = "none"

	// BuildTime is when the binary was built
	BuildTime = "unknown"

	// BuildHash is a unique hash for this build
	BuildHash = "none"

	// GoVersion is the Go version used to build
	GoVersion = "unknown"

	// GOOS is the target operating system
	GOOS = "unknown"

	// GOARCH is the target architecture
	GOARCH = "unknown"
)

// VersionInfo contains all version metadata.
type VersionInfo struct {
	Version   string `json:"version"`
	GitHash   string `json:"git_hash"`
	BuildTime string `json:"build_time"`

	BuildHash string `json:"build_hash"`
	GoVersion string `json:"go_version"`
	GoOS      string `json:"goos"`
	GoArch    string `json:"goarch"`
}

// versionCmd represents the version command.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long: `Display version information including:
	- Application version
	- Git commit hash
	- Build time
	- Build hash
	- Go version
	- OS/Architecture`,
	Run: runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output version info as JSON")
}

func runVersion(cmd *cobra.Command, _ []string) {
	out := cmd.OutOrStdout()
	if jsonOutput {
		_, _ = fmt.Fprintln(out, GetVersionJSON())
	} else {
		printVersion(out)
	}
}

// GetVersionInfo returns the version information.
func GetVersionInfo() *VersionInfo {
	return &VersionInfo{
		Version:   Version,
		GitHash:   GitHash,
		BuildTime: BuildTime,

		BuildHash: BuildHash,
		GoVersion: GoVersion,
		GoOS:      GOOS,
		GoArch:    GOARCH,
	}
}

// GetVersionJSON returns the version information as a JSON string.
func GetVersionJSON() string {
	data, err := json.MarshalIndent(GetVersionInfo(), "", "  ")
	if err != nil {
		return "{}"
	}

	return string(data)
}

func printVersion(out io.Writer) {
	_, _ = fmt.Fprintf(out, "Version:    %s\n", Version)
	_, _ = fmt.Fprintf(out, "Git Hash:   %s\n", GitHash)
	_, _ = fmt.Fprintf(out, "Build Time: %s\n", BuildTime)

	_, _ = fmt.Fprintf(out, "Build Hash: %s\n", BuildHash)
	_, _ = fmt.Fprintf(out, "Go Version: %s\n", GoVersion)
	_, _ = fmt.Fprintf(out, "OS/Arch:    %s/%s\n", GOOS, GOARCH)
}
