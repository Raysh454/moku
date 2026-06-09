package app_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/raysh454/moku/internal/app"
)

// ensureEnvUnset guarantees key is absent for the duration of the test while
// still restoring any prior value afterwards (t.Setenv registers the
// restore; os.Unsetenv models the unset case).
func ensureEnvUnset(t *testing.T, key string) {
	t.Helper()
	if prior, present := os.LookupEnv(key); present {
		t.Setenv(key, prior)
	}
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("os.Unsetenv(%s): %v", key, err)
	}
}

// The Go client must locate the sidecar through the same
// MOKU_ANALYZER_HOST/MOKU_ANALYZER_PORT family that scripts and Make use to
// bind it, so the two sides cannot disagree about the address.
func TestDefaultConfig_SidecarBaseURLFromEnv(t *testing.T) {
	cases := []struct {
		name           string
		host           string
		setHost        bool
		port           string
		setPort        bool
		wantBaseURL    string
		wantWarningLog bool
	}{
		{
			name:        "unset_defaults_to_loopback_8181",
			wantBaseURL: "http://127.0.0.1:8181",
		},
		{
			name:        "port_override_is_honored",
			setPort:     true,
			port:        "9999",
			wantBaseURL: "http://127.0.0.1:9999",
		},
		{
			name:        "host_override_is_honored",
			setHost:     true,
			host:        "analyzer.internal",
			wantBaseURL: "http://analyzer.internal:8181",
		},
		{
			name:        "host_and_port_override_combined",
			setHost:     true,
			host:        "10.1.2.3",
			setPort:     true,
			port:        "9000",
			wantBaseURL: "http://10.1.2.3:9000",
		},
		{
			name:        "bind_all_ipv4_host_maps_to_loopback",
			setHost:     true,
			host:        "0.0.0.0",
			wantBaseURL: "http://127.0.0.1:8181",
		},
		{
			name:        "bind_all_ipv6_host_maps_to_loopback",
			setHost:     true,
			host:        "::",
			wantBaseURL: "http://127.0.0.1:8181",
		},
		{
			name:        "ipv6_loopback_host_is_bracketed",
			setHost:     true,
			host:        "::1",
			wantBaseURL: "http://[::1]:8181",
		},
		{
			name:        "surrounding_whitespace_trimmed",
			setHost:     true,
			host:        "  analyzer.internal  ",
			wantBaseURL: "http://analyzer.internal:8181",
		},
		{
			name:           "non_numeric_port_warns_and_defaults",
			setPort:        true,
			port:           "not-a-port",
			wantBaseURL:    "http://127.0.0.1:8181",
			wantWarningLog: true,
		},
		{
			name:           "out_of_range_port_warns_and_defaults",
			setPort:        true,
			port:           "70000",
			wantBaseURL:    "http://127.0.0.1:8181",
			wantWarningLog: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ensureEnvUnset(t, app.EnvAnalyzerHost)
			ensureEnvUnset(t, app.EnvAnalyzerPort)
			if tc.setHost {
				t.Setenv(app.EnvAnalyzerHost, tc.host)
			}
			if tc.setPort {
				t.Setenv(app.EnvAnalyzerPort, tc.port)
			}

			var logBuf bytes.Buffer
			restoreLogOutput := captureStandardLogger(&logBuf)
			defer restoreLogOutput()

			cfg := app.DefaultConfig()

			if got := cfg.AnalyzerCfg.Sidecar.BaseURL; got != tc.wantBaseURL {
				t.Errorf("Sidecar.BaseURL = %q, want %q", got, tc.wantBaseURL)
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

// The sidecar rejects requests without a matching X-Moku-Token once
// MOKU_ANALYZER_TOKEN is set, so DefaultConfig must hand that token to the
// client as the shared secret.
func TestDefaultConfig_SidecarSharedSecretFromEnv(t *testing.T) {
	t.Run("unset_leaves_secret_empty", func(t *testing.T) {
		ensureEnvUnset(t, app.EnvAnalyzerToken)

		cfg := app.DefaultConfig()

		if got := cfg.AnalyzerCfg.Sidecar.SharedSecret; got != "" {
			t.Errorf("Sidecar.SharedSecret = %q, want empty", got)
		}
	})

	t.Run("set_token_becomes_shared_secret", func(t *testing.T) {
		t.Setenv(app.EnvAnalyzerToken, "super-secret-token")

		cfg := app.DefaultConfig()

		if got := cfg.AnalyzerCfg.Sidecar.SharedSecret; got != "super-secret-token" {
			t.Errorf("Sidecar.SharedSecret = %q, want %q", got, "super-secret-token")
		}
	})
}

func TestDefaultConfig_StorageRootFromEnv(t *testing.T) {
	t.Run("unset_defaults_to_config_moku", func(t *testing.T) {
		ensureEnvUnset(t, app.EnvStorageRoot)

		cfg := app.DefaultConfig()

		if got := cfg.StorageRoot; got != "~/.config/moku" {
			t.Errorf("StorageRoot = %q, want %q", got, "~/.config/moku")
		}
	})

	t.Run("set_value_overrides_default", func(t *testing.T) {
		t.Setenv(app.EnvStorageRoot, "/srv/moku-data")

		cfg := app.DefaultConfig()

		if got := cfg.StorageRoot; got != "/srv/moku-data" {
			t.Errorf("StorageRoot = %q, want %q", got, "/srv/moku-data")
		}
	})

	t.Run("whitespace_only_value_falls_back_to_default", func(t *testing.T) {
		t.Setenv(app.EnvStorageRoot, "   ")

		cfg := app.DefaultConfig()

		if got := cfg.StorageRoot; got != "~/.config/moku" {
			t.Errorf("StorageRoot = %q, want %q", got, "~/.config/moku")
		}
	})
}
