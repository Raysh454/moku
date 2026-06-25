// Command grade runs the webclient grading harness against a panel of
// bot-detection / Cloudflare-challenge probes and prints a scorecard, so a
// fetch backend can be compared empirically on the fast / undetectable axes.
//
// Usage:
//
//	go run ./cmd/grade [flags]
//	go run ./cmd/grade -backend nethttp -repeats 5 -format text
//	go run ./cmd/grade -backend chromedp -url https://example.com
//
// Only grade targets you are authorized to access.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/raysh454/moku/internal/grading"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

func main() {
	backend := flag.String("backend", "nethttp", "webclient backend to grade (nethttp, tls, chromedp)")
	repeats := flag.Int("repeats", 3, "samples taken per probe")
	format := flag.String("format", "text", "report format (text, json)")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	url := flag.String("url", "", "grade a single URL instead of the default panel")
	allowPrivate := flag.Bool("allow-private", false, "allow fetching private/loopback hosts (SSRF guard off)")
	flag.Parse()

	if err := run(*backend, *repeats, *format, *timeout, *url, *allowPrivate); err != nil {
		fmt.Fprintf(os.Stderr, "grade: %v\n", err)
		os.Exit(1)
	}
}

func run(backend string, repeats int, format string, timeout time.Duration, url string, allowPrivate bool) error {
	logger := logging.NewStdoutLogger("grade")

	client, err := webclient.NewWebClient(webclient.Config{
		Client:            webclient.Client(backend),
		AllowPrivateHosts: allowPrivate,
	}, logger)
	if err != nil {
		return fmt.Errorf("constructing %q backend: %w", backend, err)
	}
	defer func() { _ = client.Close() }()

	grader := grading.NewGrader(client, grading.GraderConfig{
		Classifier:        grading.DefaultClassifier(),
		Repeats:           repeats,
		Backend:           backend,
		PerRequestTimeout: timeout,
	}, logger)

	card := grader.Grade(context.Background(), panel(url))

	return render(card, format)
}

// panel returns a single-URL panel when url is set, otherwise the default panel.
func panel(url string) []grading.Probe {
	if url != "" {
		return []grading.Probe{{Name: url, URL: url}}
	}
	return grading.DefaultPanel()
}

// render writes the scorecard in the requested format.
func render(card grading.Scorecard, format string) error {
	switch format {
	case "json":
		data, err := grading.JSONReport(card)
		if err != nil {
			return fmt.Errorf("rendering json: %w", err)
		}
		fmt.Println(string(data))
	case "text":
		fmt.Print(grading.TextReport(card))
	default:
		return fmt.Errorf("unknown format %q (want text or json)", format)
	}
	return nil
}
