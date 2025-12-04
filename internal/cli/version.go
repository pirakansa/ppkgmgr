package cli

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

const defaultVersion = "0.0.0"

// Version is set by the main package prior to executing the CLI.
var Version = defaultVersion

var buildInfoReader = debug.ReadBuildInfo

// newVersionCmd reports CLI version details.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ver",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "unexpected arguments")
				return cliError{Code: 1}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Version : %s\n", resolvedVersion())
			return nil
		},
	}
}

func resolvedVersion() string {
	if v := normalizedVersion(Version); v != "" {
		return v
	}

	if info, ok := buildInfoReader(); ok {
		if v := normalizedVersion(info.Main.Version); v != "" {
			return v
		}
	}

	return defaultVersion
}

func normalizedVersion(v string) string {
	version := strings.TrimSpace(v)
	if version == "" || version == "(devel)" || version == defaultVersion {
		return ""
	}
	return version
}
