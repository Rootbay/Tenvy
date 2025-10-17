package agent

import "testing"

func TestCanonicalizeServerURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "localhost with port",
			input: "http://localhost:2332",
			want:  "http://127.0.0.1:2332",
		},
		{
			name:  "localhost without port",
			input: "https://localhost",
			want:  "https://127.0.0.1",
		},
		{
			name:  "custom host",
			input: "https://controller.example.com",
			want:  "https://controller.example.com",
		},
		{
			name:  "ipv6 loopback",
			input: "http://[::1]:8080",
			want:  "http://[::1]:8080",
		},
		{
			name:    "invalid url",
			input:   "controller",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := canonicalizeServerURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("canonicalizeServerURL returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("canonicalizeServerURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
