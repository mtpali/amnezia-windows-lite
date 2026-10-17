// SPDX-License-Identifier: MIT
package preferences

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	LastTunnel string `json:"last_connected_tunnel"`
}

func Load(path string) State {
	var state State
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &state)
	}
	return state
}

func (s State) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// Prefer an existing selection, then the last successful connection. The first
// row is only a fallback when neither name exists (for example, after deletion).
func PreferredTunnel(names []string, current, remembered string) string {
	for _, want := range []string{current, remembered} {
		if want == "" {
			continue
		}
		for _, name := range names {
			if name == want {
				return name
			}
		}
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
}
