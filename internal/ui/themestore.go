package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// fileConfig is the whole on-disk file. The theme used to be the only key,
// which is why the file was an object from the start; the scan interval is the
// second, and it is tagged omitempty so a default interval still writes
// exactly {"theme":"nord"}. This is still not a general config system.
type fileConfig struct {
	Theme                  string `json:"theme"`
	RefreshIntervalSeconds int    `json:"refreshIntervalSeconds,omitempty"`
}

// knownConfigKeys is the whole schema. A key outside it is reported rather
// than silently dropped, so a typo in a hand-edited file is visible.
var knownConfigKeys = map[string]bool{"theme": true, "refreshIntervalSeconds": true}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "procdeck", "config.json"), nil
}

// loadConfig reads the file and returns a usable config no matter what it
// finds. A missing file is the normal first run and warns about nothing; every
// other problem degrades to a default for that one setting and returns a
// warning for the caller to show. Nothing here is fatal, so a broken config
// file can never stop procdeck starting.
//
// Only one warning fits on the status line, so the later a problem is detected
// the more it has to say for itself: a structural failure beats a bad theme,
// which beats a bad interval, which beats an unknown key.
func loadConfig() (fileConfig, string) {
	cfg := fileConfig{Theme: defaultThemeName, RefreshIntervalSeconds: defaultRefreshSeconds}
	fallback := func(format string, args ...any) (fileConfig, string) {
		return cfg, fmt.Sprintf(format, args...) + ", using the " + defaultThemeName + " theme"
	}

	path, err := configPath()
	if err != nil {
		return fallback("could not locate the config directory: %v", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, ""
	}
	if err != nil {
		return fallback("could not read %s: %v", path, err)
	}
	var onDisk fileConfig
	if err := json.Unmarshal(data, &onDisk); err != nil {
		return fallback("could not read %s: %v", path, err)
	}
	// a second decode, by key, is what makes an unknown setting detectable and
	// what tells an explicit "refreshIntervalSeconds": 0 apart from an absent
	// key, since both decode to the same zero value
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fallback("could not read %s: %v", path, err)
	}

	warning := ""
	for key := range raw {
		if !knownConfigKeys[key] {
			// map order is random, so name one key rather than a shuffled list
			warning = fmt.Sprintf("ignoring unknown setting %q in %s", key, path)
			break
		}
	}
	if onDisk.Theme != "" {
		if ThemeExists(onDisk.Theme) {
			cfg.Theme = onDisk.Theme
		} else {
			warning = fmt.Sprintf("unknown theme %q in %s, using the %s theme",
				onDisk.Theme, path, defaultThemeName)
		}
	}
	if _, set := raw["refreshIntervalSeconds"]; set {
		n := onDisk.RefreshIntervalSeconds
		if n < minRefreshSeconds || n > maxRefreshSeconds {
			warning = fmt.Sprintf("refreshIntervalSeconds %d in %s is out of range, scanning every %ds",
				n, path, defaultRefreshSeconds)
		} else {
			cfg.RefreshIntervalSeconds = n
		}
	}
	return cfg, warning
}

// LoadConfig reads every remembered setting for the startup path.
func LoadConfig() (theme string, seconds int, warning string) {
	cfg, warning := loadConfig()
	return cfg.Theme, cfg.RefreshIntervalSeconds, warning
}

// LoadTheme reads the remembered theme name only.
func LoadTheme() (name, warning string) {
	cfg, warning := loadConfig()
	return cfg.Theme, warning
}

// saveConfig is the only writer. It writes atomically: a temp file beside the
// target, then a rename over it, so an interrupted write never leaves a
// half-written config. The caller treats a returned error as "in memory only",
// not as fatal.
func saveConfig(cfg fileConfig) error {
	if cfg.RefreshIntervalSeconds == defaultRefreshSeconds {
		// an absent key already means the default, and omitempty then keeps a
		// theme-only preference at exactly {"theme":"..."}
		cfg.RefreshIntervalSeconds = 0
	}
	path, err := configPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(cfg)
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
	// without this the rename can publish a name whose contents are still in
	// the OS cache, so a crash leaves a zero-length config behind
	if err := f.Sync(); err != nil {
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

// saveTheme and saveInterval are read-modify-write wrappers so neither
// setting can clobber the other: switching a theme must not forget a
// configured scan interval, and vice versa.
func saveTheme(name string) error {
	cfg, _ := loadConfig()
	cfg.Theme = name
	return saveConfig(cfg)
}

func saveInterval(seconds int) error {
	cfg, _ := loadConfig()
	cfg.RefreshIntervalSeconds = seconds
	return saveConfig(cfg)
}
