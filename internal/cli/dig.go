package cli

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
	"github.com/zeebo/blake3"
	yaml "gopkg.in/yaml.v3"
)

// newDigCmd wires the `util dig` command for computing BLAKE3 digests.
func newDigCmd() *cobra.Command {
	var format string
	var mode string

	cmd := &cobra.Command{
		Use:   "dig <path>",
		Short: "Print the BLAKE3 digest for the specified file",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			if len(args) == 0 {
				fmt.Fprintln(stderr, "require file path argument")
				return cliError{code: 1}
			}
			if len(args) > 1 {
				fmt.Fprintln(stderr, "unexpected arguments")
				return cliError{code: 1}
			}

			path, err := expandPath(args[0])
			if err != nil {
				fmt.Fprintf(stderr, "failed to expand path: %v\n", err)
				return cliError{code: 5}
			}
			if path == "" {
				fmt.Fprintln(stderr, "require file path argument")
				return cliError{code: 1}
			}

			switch strings.ToLower(mode) {
			case "file":
				return digFile(stdout, stderr, path, format)
			case "artifact":
				return digArtifact(stdout, stderr, path, format)
			default:
				fmt.Fprintf(stderr, "invalid mode %q (expected file or artifact)\n", mode)
				return cliError{code: 1}
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "raw", "output format: raw or yaml")
	cmd.Flags().StringVar(&mode, "mode", "file", "input mode: file (default) or artifact")
	return cmd
}

func digFile(stdout, stderr io.Writer, path, format string) error {
	_, digest, err := verifyDigest(path, "")
	if err != nil {
		fmt.Fprintf(stderr, "failed to compute digest: %v\n", err)
		return cliError{code: 5}
	}

	switch strings.ToLower(format) {
	case "raw":
		fmt.Fprintln(stdout, digest)
	case "yaml":
		if err := writeDigestYAML(stdout, digestFileSnippet{
			FileName: filepath.Base(path),
			OutDir:   filepath.Dir(path),
			Digest:   digest,
		}); err != nil {
			fmt.Fprintf(stderr, "failed to render yaml: %v\n", err)
			return cliError{code: 5}
		}
	default:
		fmt.Fprintf(stderr, "invalid format %q (expected raw or yaml)\n", format)
		return cliError{code: 1}
	}

	return nil
}

func digArtifact(stdout, stderr io.Writer, path, format string) error {
	_, artifactDigest, err := verifyDigest(path, "")
	if err != nil {
		fmt.Fprintf(stderr, "failed to compute artifact digest: %v\n", err)
		return cliError{code: 5}
	}

	contentDigest, err := digestZstdContent(path)
	if err != nil {
		fmt.Fprintf(stderr, "failed to compute decoded digest: %v\n", err)
		return cliError{code: 5}
	}

	switch strings.ToLower(format) {
	case "raw":
		fmt.Fprintln(stdout, artifactDigest)
	case "yaml":
		if err := writeDigestYAML(stdout, digestFileSnippet{
			FileName:       filepath.Base(path),
			OutDir:         filepath.Dir(path),
			Digest:         contentDigest,
			ArtifactDigest: artifactDigest,
			Encoding:       "zstd",
		}); err != nil {
			fmt.Fprintf(stderr, "failed to render yaml: %v\n", err)
			return cliError{code: 5}
		}
	default:
		fmt.Fprintf(stderr, "invalid format %q (expected raw or yaml)\n", format)
		return cliError{code: 1}
	}

	return nil
}

type digestSnippet struct {
	Files []digestFileSnippet `yaml:"files"`
}

type digestFileSnippet struct {
	FileName       string `yaml:"file_name"`
	OutDir         string `yaml:"out_dir"`
	Digest         string `yaml:"digest,omitempty"`
	ArtifactDigest string `yaml:"artifact_digest,omitempty"`
	Encoding       string `yaml:"encoding,omitempty"`
}

func writeDigestYAML(stdout io.Writer, file digestFileSnippet) error {
	payload := digestSnippet{Files: []digestFileSnippet{file}}
	raw, err := yaml.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = stdout.Write(raw)
	return err
}

func digestZstdContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer file.Close()

	decoder, err := zstd.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("init decoder: %w", err)
	}
	defer decoder.Close()

	hasher := blake3.New()
	if _, err := io.Copy(hasher, decoder); err != nil {
		return "", fmt.Errorf("decode artifact: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
