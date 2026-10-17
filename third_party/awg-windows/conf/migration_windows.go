/* SPDX-License-Identifier: MIT
 * Copyright (C) 2019-2021 WireGuard LLC. All Rights Reserved.
 */

package conf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var migrating sync.Mutex

type MigrationCallback func(name, oldPath, newPath string) error

// MigrateEncryptedConfigs upgrades legacy DPAPI files on the original Windows
// installation. A conflicting or unreadable file is preserved. The encrypted
// original is removed only after verifying the plaintext and updating any
// installed tunnel service. Ordinary .conf files are never encrypted or removed.
func MigrateEncryptedConfigs(migrated MigrationCallback) error {
	migrating.Lock()
	defer migrating.Unlock()
	dir, err := tunnelConfigurationsDirectory()
	if err != nil {
		return err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var failures []error
	for _, file := range files {
		if !file.Type().IsRegular() || !PathIsEncrypted(file.Name()) {
			continue
		}
		oldPath := filepath.Join(dir, file.Name())
		if err := migrateEncryptedConfig(oldPath, dir, migrated); err != nil {
			failures = append(failures, fmt.Errorf("migrate %s: %w", file.Name(), err))
		}
	}
	return errors.Join(failures...)
}

func migrateEncryptedConfig(oldPath, dir string, migrated MigrationCallback) error {
	config, err := LoadFromPath(oldPath)
	if err != nil {
		return err
	}
	newPath := filepath.Join(dir, config.Name+configFileSuffix)
	if err := config.Save(false); err != nil && !os.IsExist(err) {
		return err
	}
	// Also handles an interrupted migration that already wrote the .conf file.
	stored, err := LoadFromPath(newPath)
	if err != nil {
		return err
	}
	if stored.ToWgQuick() != config.ToWgQuick() {
		return errors.New("a different plaintext configuration already exists")
	}
	if migrated != nil {
		if err := migrated(config.Name, oldPath, newPath); err != nil {
			return err
		}
	}
	return os.Remove(oldPath)
}
