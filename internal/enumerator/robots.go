package enumerator

import (
	"context"
	"strings"

	"github.com/raysh454/moku/internal/logging"
	"github.com/raysh454/moku/internal/utils"
	"github.com/raysh454/moku/internal/webclient"
)

type Robots struct {
	wc     webclient.WebClient
	logger logging.Logger
}

func NewRobots(wc webclient.WebClient, logger logging.Logger) *Robots {
	return &Robots{wc: wc, logger: logger}
}

func (r *Robots) Enumerate(ctx context.Context, target string, cb utils.ProgressCallback) ([]string, error) {
	root, err := utils.NewURLTools(target)
	if err != nil {
		return nil, err
	}

	resp, err := r.wc.Get(ctx, target+"/robots.txt")
	if err != nil {
		r.logWarn("failed to fetch robots.txt", err)
		return nil, nil
	}
	if resp.StatusCode == 404 {
		return nil, nil
	}

	paths := parseRobotsTxt(string(resp.Body))

	seen := make(map[string]struct{})
	var results []string

	for _, p := range paths {
		resolved := resolveRobotsPath(p, target, root)
		if resolved == "" {
			continue
		}
		if _, exists := seen[resolved]; exists {
			continue
		}
		seen[resolved] = struct{}{}
		results = append(results, resolved)
	}

	if cb != nil {
		cb(1, 0, 1)
	}

	return results, nil
}

func parseRobotsTxt(body string) []string {
	var paths []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		directive := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])
		if value == "" {
			continue
		}
		switch directive {
		case "disallow", "allow":
			value = strings.TrimRight(value, "*")
			if value == "" {
				continue
			}
			// Strip trailing slash for consistency, but keep root "/"
			value = strings.TrimRight(value, "/")
			if value == "" {
				value = "/"
			}
			paths = append(paths, value)
		case "sitemap":
			paths = append(paths, value)
		}
	}
	return paths
}

func resolveRobotsPath(path, target string, root *utils.URLTools) string {
	// Absolute URL (sitemap directive)
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		parsed, err := utils.NewURLTools(path)
		if err != nil {
			return ""
		}
		if !root.DomainIsSame(parsed) {
			return ""
		}
		return path
	}
	// Relative path
	return target + path
}

func (r *Robots) logWarn(msg string, err error) {
	if r.logger != nil {
		r.logger.Warn(msg, logging.Field{Key: "error", Value: err})
	}
}
