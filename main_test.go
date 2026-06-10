package main

import "testing"

func TestResolveListenAddr(t *testing.T) {
	emptyEnv := func(string) string { return "" }

	cases := []struct {
		name    string
		args    []string
		getenv  func(string) string
		want    string
		wantErr bool
	}{
		{
			name:   "no args and no env yields loopback default",
			args:   nil,
			getenv: emptyEnv,
			want:   "127.0.0.1:8080",
		},
		{
			name: "env host and port override default",
			args: nil,
			getenv: func(key string) string {
				switch key {
				case "MOKU_HOST":
					return "0.0.0.0"
				case "MOKU_PORT":
					return "9090"
				default:
					return ""
				}
			},
			want: "0.0.0.0:9090",
		},
		{
			name: "positional args override env",
			args: []string{"10.0.0.5", "1234"},
			getenv: func(key string) string {
				switch key {
				case "MOKU_HOST":
					return "0.0.0.0"
				case "MOKU_PORT":
					return "9090"
				default:
					return ""
				}
			},
			want: "10.0.0.5:1234",
		},
		{
			name:    "invalid positional port is rejected",
			args:    []string{"127.0.0.1", "notaport"},
			getenv:  emptyEnv,
			wantErr: true,
		},
		{
			name: "invalid env port is rejected",
			args: nil,
			getenv: func(key string) string {
				if key == "MOKU_PORT" {
					return "70000"
				}
				return ""
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveListenAddr(tc.args, tc.getenv)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("resolveListenAddr(%v) expected error, got addr %q", tc.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveListenAddr(%v) unexpected error: %v", tc.args, err)
			}
			if got != tc.want {
				t.Errorf("resolveListenAddr(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}
