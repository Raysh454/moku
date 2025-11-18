package cli

import (
	"flag"
	"fmt"
	"strings"
)

// CLIArgs are the command-line arguments that control a single-run or a job.
// Keep this small for now — add fields as modules need them.
type CLIArgs struct {
	// Target is the root URL (or host) to enumerate/fetch.
	Target string

	// JobType is optional; if empty the CLI will pick a default (e.g. full-scan).
	JobType string

	// Concurrency overrides for the fetcher for this run; 0 means "use config default".
	Concurrency int

	// RawArgs is the original args slice (useful for debugging/tests).
	RawArgs []string
}

// ParseArgs parses a slice of args and returns CLIArgs. Use in tests by passing
// arbitrary slices. The function is deterministic and does not read os.Args.
func ParseArgs(args []string) (*CLIArgs, error) {
	fs := flag.NewFlagSet("moku-cli", flag.ContinueOnError)
	var (
		target      = fs.String("target", "", "Root URL to enumerate and fetch (required)")
		jobType     = fs.String("job", "full-scan", "Job type: full-scan|fetch|enumerate|vuln-scan")
		concurrency = fs.Int("concurrency", 0, "Fetcher concurrency for this run (0=use default)")
	)

	// Ensure Parse doesn't write to stdout/stderr in tests
	fs.SetOutput(nil)

	if err := fs.Parse(args); err != nil {
		// Flag parsing errors are useful to return to caller
		return nil, err
	}

	if strings.TrimSpace(*target) == "" {
		return nil, fmt.Errorf("missing required -target argument")
	}

	return &CLIArgs{
		Target:      *target,
		JobType:     *jobType,
		Concurrency: *concurrency,
		RawArgs:     args,
	}, nil
}
