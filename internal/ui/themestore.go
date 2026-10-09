package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// themeConfig is the whole on-disk file. It is an object rather than a bare
// string so a later key would not need a migration, but the theme name is the
// only thing procdeck persists — this is not a general config system.
type themeConfig struct {
	Theme string `json:"theme"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "procdeck", "config.json"), nil
}

// LoadTheme reads the remembered theme name. A missing file is the normal
// first run and returns no warning; every other failure degrades to the
// default theme and returns a warning for the caller to show. Nothing here is
// fatal, so a broken config file can never stop procdeck starting.
func LoadTheme() (name, warning string) {
	fallback := func(format string, args ...any) (string, string) {
		return defaultThemeName, fmt.Sprintf(format, args...) + ", using the " + defaultThemeName + " theme"
	}

	path, err := configPath()
	if err != nil {
		return fallback("could not locate the config directory: %v", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultThemeName, ""
	}
	if err != nil {
		return fallback("could not read %s: %v", path, err)
	}
	var cfg themeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fallback("could not read %s: %v", path, err)
	}
	if cfg.Theme == "" {
		return defaultThemeName, ""
	}
	if !ThemeExists(cfg.Theme) {
		return fallback("unknown theme %q in %s", cfg.Theme, path)
	}
	return cfg.Theme, ""
}

// saveTheme writes the name atomically: a temp file beside the target, then a
// rename over it, so an interrupted write never leaves a half-written config.
// The caller treats a returned error as "in memory only", not as fatal.
func saveTheme(name string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(themeConfig{Theme: name})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "config-*.json.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(append(data, '\n')); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
