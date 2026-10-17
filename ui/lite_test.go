package ui

import "testing"

func TestLiteTrafficLabels(t *testing.T) {
	for _, tc := range []struct {
		n    uint64
		want string
	}{{0, "0 B"}, {1023, "1023 B"}, {1024, "1.0 KB"}, {1048576, "1.0 MB"}, {1 << 40, "1.0 TB"}} {
		if got := formatBytes(tc.n); got != tc.want {
			t.Fatalf("%d: got %q, want %q", tc.n, got, tc.want)
		}
	}
}
