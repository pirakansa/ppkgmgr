package shared

import "errors"

// DownloadFunc downloads a remote file into the provided destination.
type DownloadFunc func(string, string) (int64, error)

// Error represents structured CLI failures that map to exit codes.
type Error struct {
	Code int
}

func (e Error) Error() string {
	return "cli error"
}

// ExitCode converts structured CLI errors into process exit codes.
func ExitCode(err error) int {
	var cliErr Error
	if errors.As(err, &cliErr) {
		return cliErr.Code
	}
	return 1
}
