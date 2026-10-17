// SPDX-License-Identifier: MIT
package ui

import (
	"log"
	"path/filepath"

	"github.com/mtpali/amnezia-windows-lite/internal/preferences"
	"github.com/mtpali/amnezia-windows-lite/portable"
)

var uiPreferences preferences.State

func preferencesPath() string {
	dir, err := portable.Directory()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "State", "ui.json")
}

func rememberConnectedTunnel(name string) {
	if name == "" || uiPreferences.LastTunnel == name {
		return
	}
	uiPreferences.LastTunnel = name
	if err := uiPreferences.Save(preferencesPath()); err != nil {
		log.Printf("Saving last connected tunnel: %v", err)
	}
}
