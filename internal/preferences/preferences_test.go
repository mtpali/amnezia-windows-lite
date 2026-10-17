package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectionSurvivesReload(t *testing.T) {
	for _, tc := range []struct{ current, last, want string }{
		{"", "Zulu", "Zulu"},
		{"Beta", "Zulu", "Beta"},
		{"deleted", "Zulu", "Zulu"},
		{"deleted", "missing", "Alpha"},
		{"", "", "Alpha"},
	} {
		if got := PreferredTunnel([]string{"Alpha", "Beta", "Zulu"}, tc.current, tc.last); got != tc.want {
			t.Fatalf("current=%q remembered=%q: got %q, want %q", tc.current, tc.last, got, tc.want)
		}
	}
	if got := PreferredTunnel(nil, "deleted", "deleted"); got != "" {
		t.Fatal(got)
	}
}

func TestStateRoundTripAndOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "State", "ui.json")
	for _, name := range []string{"Zulu", "سرور ۲"} {
		if err := (State{LastTunnel: name}).Save(path); err != nil {
			t.Fatal(err)
		}
		if got := Load(path).LastTunnel; got != name {
			t.Fatalf("got %q, want %q", got, name)
		}
	}
	if err := os.WriteFile(path, []byte("broken json"), 0600); err != nil {
		t.Fatal(err)
	}
	if Load(path).LastTunnel != "" {
		t.Fatal("corrupt state should fall back to an empty selection")
	}
}
