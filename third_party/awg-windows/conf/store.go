/* SPDX-License-Identifier: MIT
 *
 * Copyright (C) 2019-2021 WireGuard LLC. All Rights Reserved.
 */

package conf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/amnezia-vpn/amneziawg-windows/v3/conf/dpapi"
)

const configFileSuffix = ".conf"
const configFileEncryptedSuffix = ".conf.dpapi"

func ListConfigNames() ([]string, error) {
	configFileDir, err := tunnelConfigurationsDirectory()
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(configFileDir)
	if err != nil {
		return nil, err
	}
	configs := make([]string, 0, len(files))
	seen := make(map[string]bool)
	for _, file := range files {
		name := filepath.Base(file.Name())
		name, err = NameFromPath(name)
		if err != nil {
			continue
		}
		if !file.Type().IsRegular() {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue
		}
		if info.Mode().Perm()&0444 == 0 {
			continue
		}
		key := strings.ToLower(name)
		if !seen[key] {
			configs = append(configs, name)
			seen[key] = true
		}
	}
	return configs, nil
}

func LoadFromName(name string) (*Config, error) {
	path, err := (&Config{Name: name}).Path()
	if err != nil {
		return nil, err
	}
	return LoadFromPath(path)
}

func LoadFromPath(path string) (*Config, error) {
	name, err := NameFromPath(path)
	if err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if PathIsEncrypted(path) {
		bytes, err = dpapi.Decrypt(bytes, name)
		if err != nil {
			return nil, err
		}
	}
	return FromWgQuickWithUnknownEncoding(string(bytes), name)
}

func PathIsEncrypted(path string) bool {
	return strings.HasSuffix(filepath.Base(path), configFileEncryptedSuffix)
}

func NameFromPath(path string) (string, error) {
	name := filepath.Base(path)
	switch {
	case strings.HasSuffix(name, configFileEncryptedSuffix):
		name = strings.TrimSuffix(name, configFileEncryptedSuffix)
	case strings.HasSuffix(name, configFileSuffix):
		name = strings.TrimSuffix(name, configFileSuffix)
	default:
		return "", errors.New("Path must end in either " + configFileSuffix + " or " + configFileEncryptedSuffix)
	}
	if !TunnelNameIsValid(name) {
		return "", errors.New("Tunnel name is not valid")
	}
	return name, nil
}

func (config *Config) Save(overwrite bool) error {
	if !TunnelNameIsValid(config.Name) {
		return errors.New("Tunnel name is not valid")
	}
	configFileDir, err := tunnelConfigurationsDirectory()
	if err != nil {
		return err
	}
	filename := filepath.Join(configFileDir, config.Name+configFileSuffix)
	bytes := []byte(config.ToWgQuick())
	return writeConfigFile(filename, overwrite, bytes)
}

func (config *Config) Path() (string, error) {
	if !TunnelNameIsValid(config.Name) {
		return "", errors.New("Tunnel name is not valid")
	}
	configFileDir, err := tunnelConfigurationsDirectory()
	if err != nil {
		return "", err
	}
	path := filepath.Join(configFileDir, config.Name+configFileSuffix)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		legacyPath := filepath.Join(configFileDir, config.Name+configFileEncryptedSuffix)
		if _, legacyErr := os.Stat(legacyPath); legacyErr == nil {
			return legacyPath, nil
		}
	}
	return path, nil
}

func DeleteName(name string) error {
	if !TunnelNameIsValid(name) {
		return errors.New("Tunnel name is not valid")
	}
	configFileDir, err := tunnelConfigurationsDirectory()
	if err != nil {
		return err
	}
	var removed bool
	for _, suffix := range []string{configFileSuffix, configFileEncryptedSuffix} {
		err := os.Remove(filepath.Join(configFileDir, name+suffix))
		if err == nil {
			removed = true
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if !removed {
		return os.ErrNotExist
	}
	return nil
}

func (config *Config) Delete() error {
	return DeleteName(config.Name)
}
