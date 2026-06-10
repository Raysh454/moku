package app_test

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/raysh454/moku/internal/analyzer"
	"github.com/raysh454/moku/internal/app"
)

// TestDefaultConfig_AnalyzerBackendFromEnv exercises the
// MOKU_ANALYZER_BACKEND env override exposed through app.EnvAnalyzerBackend.
// The matrix verifies that:
//   - every documented value selects the matching analyzer.Backend constant
//     (case-insensitive, surrounding whitespace ignored),
//   - an unset value falls back to BackendMoku (existing behavior),
//   - an unrecognized value emits a warning and falls back to BackendMoku
//     rather than panicking or returning a nil-zero Backend.
func TestDefaultConfig_AnalyzerBackendFromEnv(t *testing.T) {
	cases := []struct {
		name           string
		envValue       string // empty string means leave the env var unset.
		setEnv         bool
		wantBackend    analyzer.Backend
		wantWarningLog bool
	}{
		{
			name:        "unset_defaults_to_moku",
			setEnv:      false,
			wantBackend: analyzer.BackendMoku,
		},
		{
			name:        "empty_string_defaults_to_moku",
			setEnv:      true,
			envValue:    "",
			wantBackend: analyzer.BackendMoku,
		},
		{
			name:        "moku_selected_explicitly",
			setEnv:      true,
			envValue:    "moku",
			wantBackend: analyzer.BackendMoku,
		},
		{
			name:        "dast_lowercase",
			setEnv:      true,
			envValue:    "dast",
			wantBackend: analyzer.BackendDAST,
		},
		{
			name:        "nuclei_lowercase",
			setEnv:      true,
			envValue:    "nuclei",
			wantBackend: analyzer.BackendNuclei,
		},
		{
			name:        "nikto_lowercase",
			setEnv:      true,
			envValue:    "nikto",
			wantBackend: analyzer.BackendNikto,
		},
		{
			name:        "shodan_lowercase",
			setEnv:      true,
			envValue:    "shodan",
			wantBackend: analyzer.BackendShodan,
		},
		{
			name:        "virustotal_lowercase",
			setEnv:      true,
			envValue:    "virustotal",
			wantBackend: analyzer.BackendVirusTotal,
		},
		{
			name:           "burp_removed_warns_and_defaults_to_moku",
			setEnv:         true,
			envValue:       "burp",
			wantBackend:    analyzer.BackendMoku,
			wantWarningLog: true,
		},
		{
			name:        "zap_lowercase",
			setEnv:      true,
			envValue:    "zap",
			wantBackend: analyzer.BackendZAP,
		},
		{
			name:        "case_insensitive_uppercase",
			setEnv:      true,
			envValue:    "NUCLEI",
			wantBackend: analyzer.BackendNuclei,
		},
		{
			name:        "case_insensitive_mixed_case",
			setEnv:      true,
			envValue:    "VirusTotal",
			wantBackend: analyzer.BackendVirusTotal,
		},
		{
			name:        "surrounding_whitespace_trimmed",
			setEnv:      true,
			envValue:    "  dast  ",
			wantBackend: analyzer.BackendDAST,
		},
		{
			name:           "unknown_value_warns_and_defaults_to_moku",
			setEnv:         true,
			envValue:       "totally-bogus-backend",
			wantBackend:    analyzer.BackendMoku,
			wantWarningLog: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(app.EnvAnalyzerBackend, tc.envValue)
			} else {
				// t.Setenv only records-and-restores when called, so we must
				// explicitly unset the variable to model the unset case.
				// Registering the prior value with t.Setenv first ensures
				// the test framework restores it after the test even when
				// it was originally set in the test process environment.
				if prior, present := os.LookupEnv(app.EnvAnalyzerBackend); present {
					t.Setenv(app.EnvAnalyzerBackend, prior)
				}
				if err := os.Unsetenv(app.EnvAnalyzerBackend); err != nil {
					t.Fatalf("os.Unsetenv: %v", err)
				}
			}

			var logBuf bytes.Buffer
			restoreLogOutput := captureStandardLogger(&logBuf)
			defer restoreLogOutput()

			cfg := app.DefaultConfig()

			if cfg == nil {
				t.Fatal("DefaultConfig() returned nil")
			}
			if got := cfg.AnalyzerCfg.Backend; got != tc.wantBackend {
				t.Errorf("AnalyzerCfg.Backend = %q, want %q", got, tc.wantBackend)
			}

			loggedWarning := logBuf.Len() > 0
			if loggedWarning != tc.wantWarningLog {
				t.Errorf(
					"warning emitted = %v (log=%q), want %v",
					loggedWarning, logBuf.String(), tc.wantWarningLog,
				)
			}
		})
	}
}

// TestDefaultConfig_AllowPrivateHostsFromEnv exercises the
// MOKU_ALLOW_PRIVATE_HOSTS env override exposed through
// app.EnvAllowPrivateHosts. The matrix mirrors the sidecar's truthy
// convention: "1"/"true"/"yes" (case-insensitive, whitespace-trimmed) enable
// the escape hatch; anything else — including unset and unrecognized values —
// keeps the SSRF guard engaged (the secure default).
func TestDefaultConfig_AllowPrivateHostsFromEnv(t *testing.T) {
	cases := []struct {
		name     string
		envValue string
		setEnv   bool
		want     bool
	}{
		{name: "unset_keeps_guard", setEnv: false, want: false},
		{name: "empty_keeps_guard", setEnv: true, envValue: "", want: false},
		{name: "one_enables", setEnv: true, envValue: "1", want: true},
		{name: "true_enables", setEnv: true, envValue: "true", want: true},
		{name: "yes_enables", setEnv: true, envValue: "yes", want: true},
		{name: "true_uppercase_enables", setEnv: true, envValue: "TRUE", want: true},
		{name: "yes_whitespace_enables", setEnv: true, envValue: "  yes  ", want: true},
		{name: "zero_keeps_guard", setEnv: true, envValue: "0", want: false},
		{name: "false_keeps_guard", setEnv: true, envValue: "false", want: false},
		{name: "bogus_keeps_guard", setEnv: true, envValue: "maybe", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(app.EnvAllowPrivateHosts, tc.envValue)
			} else {
				if prior, present := os.LookupEnv(app.EnvAllowPrivateHosts); present {
					t.Setenv(app.EnvAllowPrivateHosts, prior)
				}
				if err := os.Unsetenv(app.EnvAllowPrivateHosts); err != nil {
					t.Fatalf("os.Unsetenv: %v", err)
				}
			}

			cfg := app.DefaultConfig()
			if cfg == nil {
				t.Fatal("DefaultConfig() returned nil")
			}
			if got := cfg.WebClientCfg.AllowPrivateHosts; got != tc.want {
				t.Errorf("WebClientCfg.AllowPrivateHosts = %v, want %v", got, tc.want)
			}
		})
	}
}

// captureStandardLogger redirects the default log package output to dst and
// returns a restore func. Used to assert that invalid env values produce a
// warning without leaking the warning into test output.
func captureStandardLogger(dst *bytes.Buffer) func() {
	priorOutput := log.Writer()
	priorFlags := log.Flags()
	priorPrefix := log.Prefix()
	log.SetOutput(dst)
	return func() {
		log.SetOutput(priorOutput)
		log.SetFlags(priorFlags)
		log.SetPrefix(priorPrefix)
	}
}
