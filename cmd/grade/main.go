// Command grade runs the webclient grading harness against a panel of
// bot-detection / Cloudflare-challenge probes and prints a scorecard, so a
// fetch backend can be compared empirically on the fast / undetectable axes.
//
// Usage:
//
//	go run ./cmd/grade [flags]
//	go run ./cmd/grade -backend tls -repeats 5 -format text
//	go run ./cmd/grade -compare                      # benchmark every backend
//	go run ./cmd/grade -compare -remote-endpoint http://127.0.0.1:8191/v1
//
// Only grade targets you are authorized to access.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/raysh454/moku/internal/grading"
	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/webclient"
)

// options holds the parsed command-line flags.
type options struct {
	backend        string
	repeats        int
	format         string
	timeout        time.Duration
	url            string
	allowPrivate   bool
	compare        bool
	remoteEndpoint string
}

func main() {
	var opts options
	flag.StringVar(&opts.backend, "backend", "nethttp", "webclient backend to grade (nethttp, headers, tls, chromedp, remote, escalating)")
	flag.IntVar(&opts.repeats, "repeats", 3, "samples taken per probe")
	flag.StringVar(&opts.format, "format", "text", "report format (text, json)")
	flag.DurationVar(&opts.timeout, "timeout", 30*time.Second, "per-request timeout")
	flag.StringVar(&opts.url, "url", "", "grade a single URL instead of the default panel")
	flag.BoolVar(&opts.allowPrivate, "allow-private", false, "allow fetching private/loopback hosts (SSRF guard off)")
	flag.BoolVar(&opts.compare, "compare", false, "benchmark every constructible backend and print a comparison")
	flag.StringVar(&opts.remoteEndpoint, "remote-endpoint", "", "FlareSolverr/Byparr /v1 URL enabling the remote backend")
	flag.Parse()

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "grade: %v\n", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	logger := logging.NewStdoutLogger("grade")
	cfg := webclient.Config{Client: webclient.Client(opts.backend), AllowPrivateHosts: opts.allowPrivate}

	if opts.compare {
		return runComparison(opts, cfg, logger)
	}

	client, err := buildClient(opts.backend, opts.remoteEndpoint, cfg, logger)
	if err != nil {
		return fmt.Errorf("constructing %q backend: %w", opts.backend, err)
	}
	defer func() { _ = client.Close() }()

	grader := grading.NewGrader(client, grading.GraderConfig{
		Classifier:        grading.DefaultClassifier(),
		Repeats:           opts.repeats,
		Backend:           opts.backend,
		PerRequestTimeout: opts.timeout,
	}, logger)

	card := grader.Grade(context.Background(), panel(opts.url))
	return render(card, opts.format)
}

// runComparison benchmarks every constructible backend through the same panel
// and prints a comparison matrix.
func runComparison(opts options, cfg webclient.Config, logger logging.Logger) error {
	clients := buildComparisonClients(cfg, opts.remoteEndpoint, logger)
	if len(clients) == 0 {
		return fmt.Errorf("no backends could be constructed")
	}
	defer closeAll(clients)

	report := grading.RunBenchmark(context.Background(), clients, panel(opts.url), grading.GraderConfig{
		Classifier:        grading.DefaultClassifier(),
		Repeats:           opts.repeats,
		PerRequestTimeout: opts.timeout,
	}, logger)

	if opts.format == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("rendering json: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(grading.ComparisonTable(report))
	return nil
}

// buildComparisonClients constructs every backend that can be built in the
// current environment, skipping those that cannot (chromedp without Chrome, the
// remote backend without an endpoint).
func buildComparisonClients(cfg webclient.Config, remoteEndpoint string, logger logging.Logger) []grading.NamedClient {
	var clients []grading.NamedClient

	if c, err := webclient.NewNetHTTPClient(cfg, logger, nil); err == nil {
		clients = append(clients, grading.NamedClient{Name: "nethttp", Client: c})
	}
	if c, err := webclient.NewNetHTTPClient(cfg, logger, nil); err == nil {
		clients = append(clients, grading.NamedClient{Name: "headers", Client: webclient.NewBrowserHeaderClient(c, logger)})
	}
	if c, err := webclient.NewTLSClient(cfg, logger); err == nil {
		clients = append(clients, grading.NamedClient{Name: "tls", Client: c})
	}
	if remoteEndpoint != "" {
		if c, err := webclient.NewRemoteClient(remoteEndpoint, cfg, logger); err == nil {
			clients = append(clients, grading.NamedClient{Name: "remote", Client: c})
		}
	}
	if c, err := webclient.NewChromedpClient(cfg, logger); err == nil {
		clients = append(clients, grading.NamedClient{Name: "chromedp", Client: c})
	} else {
		logger.Warn("chromedp backend unavailable; excluded from comparison",
			logging.Field{Key: "error", Value: err.Error()})
	}

	return clients
}

func closeAll(clients []grading.NamedClient) {
	for _, named := range clients {
		_ = named.Client.Close()
	}
}

// buildClient constructs the requested backend. Most backends come from the
// webclient factory; "escalating" and "remote" are assembled here because the
// composition root is where the grading challenge detector and the unblocker
// endpoint are wired in (the webclient package must not depend on grading).
func buildClient(backend, remoteEndpoint string, cfg webclient.Config, logger logging.Logger) (webclient.WebClient, error) {
	switch backend {
	case "escalating":
		return buildEscalatingClient(cfg, logger)
	case "headers":
		inner, err := webclient.NewNetHTTPClient(cfg, logger, nil)
		if err != nil {
			return nil, err
		}
		return webclient.NewBrowserHeaderClient(inner, logger), nil
	case "remote":
		return webclient.NewRemoteClient(remoteEndpoint, cfg, logger)
	default:
		return webclient.NewWebClient(cfg, logger)
	}
}

// buildEscalatingClient wires the tiered chain: tls (fast, fingerprint-only)
// first, escalating to chromedp (real browser) when grading's Cloudflare
// classifier judges a response challenged or blocked. The chromedp tier is
// optional so the tool still runs where Chrome is unavailable.
func buildEscalatingClient(cfg webclient.Config, logger logging.Logger) (webclient.WebClient, error) {
	tlsTier, err := webclient.NewTLSClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("tls tier: %w", err)
	}
	tiers := []webclient.WebClient{tlsTier}

	if chromeTier, cerr := webclient.NewChromedpClient(cfg, logger); cerr != nil {
		logger.Warn("chromedp tier unavailable; escalating with tls tier only",
			logging.Field{Key: "error", Value: cerr.Error()})
	} else {
		tiers = append(tiers, chromeTier)
	}

	classifier := grading.DefaultClassifier()
	shouldEscalate := func(resp *webclient.Response) bool {
		switch classifier.Classify(resp).Outcome {
		case grading.OutcomeChallenged, grading.OutcomeBlocked:
			return true
		default:
			return false
		}
	}
	return webclient.NewEscalatingClient(tiers, shouldEscalate, logger)
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
