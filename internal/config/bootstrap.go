package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultPath keeps Windows and macOS portable installations independent of the caller's
// working directory. Explicit -config paths always retain their CLI semantics.
func DefaultPath(explicit, executable, workingDirectory, goos string) (string, error) {
	path := explicit
	if path == "" {
		base := workingDirectory
		if goos == "windows" || goos == "darwin" {
			base = filepath.Dir(executable)
		}
		path = filepath.Join(base, "config.yaml")
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(workingDirectory, path)
	}
	return filepath.Abs(path)
}

// Initialize writes first-run defaults without ever overwriting user settings.
// Other fields continue to inherit Config.Defaults, including disabled switching.
func Initialize(path string) (bool, error) {
	defaults := Defaults()
	seed := struct {
		HTTP    HTTPConfig    `yaml:"http"`
		Mihomo  MihomoConfig  `yaml:"mihomo"`
		Storage StorageConfig `yaml:"storage"`
	}{defaults.HTTP, defaults.Mihomo, defaults.Storage}
	data, err := yaml.Marshal(seed)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return false, fmt.Errorf("create configuration directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create config in a writable directory (or use -config): %w", err)
	}
	_, writeErr := file.Write(append([]byte("# Configure the Controller in Settings > Mihomo connection.\n# Relative data paths are resolved from this configuration file.\n# Keep config.yaml and data/ when upgrading the executable.\n"), data...))
	closeErr := file.Close()
	if writeErr != nil {
		return false, writeErr
	}
	return true, closeErr
}
