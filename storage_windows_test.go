package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amnezia-vpn/amneziawg-windows/v3/conf"
	"github.com/amnezia-vpn/amneziawg-windows/v3/conf/dpapi"
)

func liteConfigDirectory(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	conf.PresetRootDirectory(root)
	t.Cleanup(func() { conf.PresetRootDirectory("") })
	dir := filepath.Join(root, "Configurations")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func writeLegacyConfig(t *testing.T, dir string, c *conf.Config) string {
	t.Helper()
	data, err := dpapi.Encrypt([]byte(c.ToWgQuick()), c.Name)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, c.Name+".conf.dpapi")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLitePlaintextStorage(t *testing.T) {
	dir := liteConfigDirectory(t)
	c := parsedConfig(t, "")
	if err := c.Save(false); err != nil {
		t.Fatal(err)
	}
	path, err := c.Path()
	if err != nil || path != filepath.Join(dir, "sample.conf") || conf.PathIsEncrypted(path) {
		t.Fatalf("unexpected storage path: %q, %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "[Interface]") || !strings.Contains(string(data), "PrivateKey = "+c.Interface.PrivateKey.String()) {
		t.Fatalf("configuration is not readable plaintext: %v", err)
	}
	loaded, err := conf.LoadFromName(c.Name)
	if err != nil || loaded.ToWgQuick() != c.ToWgQuick() {
		t.Fatalf("plaintext round trip failed: %v", err)
	}
	c.Peers[0].Endpoint.Port++
	if err := c.Save(false); err == nil {
		t.Fatal("an existing configuration was overwritten without permission")
	}
	unchanged, _ := os.ReadFile(path)
	if !bytes.Equal(data, unchanged) {
		t.Fatal("failed save changed the existing configuration")
	}
	if err := c.Save(true); err != nil {
		t.Fatal(err)
	}
	loaded, err = conf.LoadFromName(c.Name)
	if err != nil || loaded.Peers[0].Endpoint.Port != c.Peers[0].Endpoint.Port {
		t.Fatalf("explicit overwrite failed: %v", err)
	}
	if _, err := os.Stat(path + ".dpapi"); !os.IsNotExist(err) {
		t.Fatal("a new configuration must not create an encrypted copy")
	}
	if _, err := conf.LoadFromName("../sample"); err == nil {
		t.Fatal("an invalid configuration name was accepted")
	}
	if err := conf.DeleteName(c.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("plaintext configuration was not deleted")
	}
}

func TestLiteLegacyMigrationResumes(t *testing.T) {
	dir := liteConfigDirectory(t)
	c := parsedConfig(t, "BlockUntunneledTraffic = on")
	oldPath := writeLegacyConfig(t, dir, c)
	if path, err := c.Path(); err != nil || path != oldPath {
		t.Fatalf("legacy read fallback is missing: %q, %v", path, err)
	}
	if loaded, err := conf.LoadFromName(c.Name); err != nil || loaded.ToWgQuick() != c.ToWgQuick() {
		t.Fatalf("legacy configuration cannot be read: %v", err)
	}
	simulatedFailure := errors.New("simulated service update failure")
	err := conf.MigrateEncryptedConfigs(func(name, old, next string) error {
		if name != c.Name || old != oldPath || next != filepath.Join(dir, "sample.conf") {
			t.Error("incorrect migration callback paths")
		}
		return simulatedFailure
	})
	if !errors.Is(err, simulatedFailure) {
		t.Fatalf("service migration failure was ignored: %v", err)
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("encrypted original was removed before service update succeeded")
	}
	if err := conf.MigrateEncryptedConfigs(nil); err != nil {
		t.Fatal("interrupted migration did not resume", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("verified legacy file was not retired")
	}
	loaded, err := conf.LoadFromName(c.Name)
	if err != nil || loaded.ToWgQuick() != c.ToWgQuick() || !loaded.BlocksUntunneledTraffic() {
		t.Fatalf("migration changed the configuration: %v", err)
	}
	if err := conf.MigrateEncryptedConfigs(nil); err != nil {
		t.Fatal("migration should leave ordinary .conf files untouched", err)
	}
}

func TestLiteLegacyMigrationPreservesConflicts(t *testing.T) {
	dir := liteConfigDirectory(t)
	c := parsedConfig(t, "")
	if err := c.Save(false); err != nil {
		t.Fatal(err)
	}
	c.Peers[0].Endpoint.Port++
	oldPath := writeLegacyConfig(t, dir, c)
	if err := conf.MigrateEncryptedConfigs(nil); err == nil {
		t.Fatal("conflicting plaintext should stop migration of that legacy file")
	}
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("conflicting encrypted original was lost")
	}
	loaded, err := conf.LoadFromName(c.Name)
	if err != nil || loaded.Peers[0].Endpoint.Port == c.Peers[0].Endpoint.Port {
		t.Fatalf("conflicting plaintext was overwritten: %v", err)
	}
	names, err := conf.ListConfigNames()
	if err != nil || len(names) != 1 || names[0] != c.Name {
		t.Fatalf("legacy and plaintext duplicates must appear once: %v, %v", names, err)
	}
	if err := conf.DeleteName(c.Name); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{".conf", ".conf.dpapi"} {
		if _, err := os.Stat(filepath.Join(dir, c.Name+suffix)); !os.IsNotExist(err) {
			t.Fatal("deleting a configuration left a duplicate behind")
		}
	}
}

func TestLiteLegacyMigrationPreservesUnreadableFiles(t *testing.T) {
	dir := liteConfigDirectory(t)
	badPath := filepath.Join(dir, "broken.conf.dpapi")
	badData := []byte("not a valid DPAPI configuration")
	if err := os.WriteFile(badPath, badData, 0600); err != nil {
		t.Fatal(err)
	}
	c := parsedConfig(t, "")
	goodPath := writeLegacyConfig(t, dir, c)
	if err := conf.MigrateEncryptedConfigs(nil); err == nil {
		t.Fatal("unreadable legacy file was not reported")
	}
	if actual, err := os.ReadFile(badPath); err != nil || !bytes.Equal(actual, badData) {
		t.Fatal("unreadable legacy data was changed or removed")
	}
	if _, err := os.Stat(goodPath); !os.IsNotExist(err) {
		t.Fatal("one bad legacy file prevented migration of a valid configuration")
	}
}
