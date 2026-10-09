package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// configHome points os.UserConfigDir at a temp tree on any OS: %AppData% on
// Windows, $XDG_CONFIG_HOME or $HOME/.config elsewhere.
func configHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return dir
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestThemeStoreMissingFileIsSilent(t *testing.T) {
	configHome(t)
	name, warning := LoadTheme()
	if name != defaultThemeName {
		t.Errorf("name = %q, want %q", name, defaultThemeName)
	}
	if warning != "" {
		t.Errorf("a first run warned: %q", warning)
	}
}

func TestThemeStoreRoundTrips(t *testing.T) {
	configHome(t)
	if err := saveTheme("nord"); err != nil {
		t.Fatalf("saveTheme: %v", err)
	}
	name, warning := LoadTheme()
	if name != "nord" || warning != "" {
		t.Errorf("round trip gave (%q, %q), want (\"nord\", \"\")", name, warning)
	}

	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(data)), `{"theme":"nord"}`; got != want {
		t.Errorf("file holds %s, want %s", got, want)
	}
}

// TestThemeStoreWritesAtomically checks the rename-over-existing path, which is
// the part most likely to behave differently on Windows, and that no temp file
// is left behind.
func TestThemeStoreWritesAtomically(t *testing.T) {
	configHome(t)
	if err := saveTheme("nord"); err != nil {
		t.Fatalf("first saveTheme: %v", err)
	}
	if err := saveTheme("mono"); err != nil {
		t.Fatalf("second saveTheme: %v", err)
	}
	if name, _ := LoadTheme(); name != "mono" {
		t.Errorf("second save did not take effect: %q", name)
	}

	path, err := configPath()
	if err != nil {
		t.Fatal(err)
	}
	left, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*tmp*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Errorf("temp files left behind: %v", left)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("config dir holds %d entries, want just config.json", len(entries))
	}
}

func TestThemeStoreMalformedFileFallsBack(t *testing.T) {
	configHome(t)
	path := writeConfig(t, "{this is not json")
	name, warning := LoadTheme()
	if name != defaultThemeName {
		t.Errorf("name = %q, want %q", name, defaultThemeName)
	}
	if warning == "" {
		t.Fatal("a malformed config file warned about nothing")
	}
	if !strings.Contains(warning, path) || !strings.Contains(warning, defaultThemeName) {
		t.Errorf("warning does not name the file and the fallback: %q", warning)
	}
}

func TestThemeStoreUnknownNameFallsBack(t *testing.T) {
	configHome(t)
	path := writeConfig(t, `{"theme":"solarized"}`)
	name, warning := LoadTheme()
	if name != defaultThemeName {
		t.Errorf("name = %q, want %q", name, defaultThemeName)
	}
	if !strings.Contains(warning, "solarized") || !strings.Contains(warning, path) {
		t.Errorf("warning = %q, want it to name the bad theme and the file", warning)
	}
}

func TestThemeStoreEmptyNameIsSilent(t *testing.T) {
	configHome(t)
	writeConfig(t, `{}`)
	name, warning := LoadTheme()
	if name != defaultThemeName || warning != "" {
		t.Errorf("got (%q, %q), want the default and no warning", name, warning)
	}
}

func TestThemeStoreCreatesParentDir(t *testing.T) {
	dir := configHome(t)
	if err := saveTheme("gruvbox"); err != nil {
		t.Fatalf("saveTheme: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "procdeck", "config.json")); err != nil {
		t.Errorf("config was not created: %v", err)
	}
}

// TestThemeStoreSaveFailureIsNotFatal forces MkdirAll to fail by making the
// parent of the config directory an existing regular file.
func TestThemeStoreSaveFailureIsNotFatal(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AppData", blocker)
	t.Setenv("XDG_CONFIG_HOME", blocker)
	t.Setenv("HOME", blocker)

	if err := saveTheme("nord"); err == nil {
		t.Error("saveTheme reported success with an unusable config directory")
	}
	// the same situation must still produce a usable theme, not a crash
	if name, warning := LoadTheme(); name != defaultThemeName {
		t.Errorf("LoadTheme gave (%q, %q), want the default theme", name, warning)
	}
}

func TestConfigIntervalRoundTrips(t *testing.T) {
	configHome(t)
	if err := saveInterval(7); err != nil {
		t.Fatalf("saveInterval: %v", err)
	}
	name, seconds, warning := LoadConfig()
	if name != defaultThemeName || seconds != 7 || warning != "" {
		t.Errorf("round trip gave (%q, %d, %q), want (%q, 7, \"\")",
			name, seconds, warning, defaultThemeName)
	}
}

// TestConfigKeysDoNotClobberEachOther is the read-modify-write guarantee: each
// writer touches only its own key.
func TestConfigKeysDoNotClobberEachOther(t *testing.T) {
	configHome(t)
	if err := saveInterval(11); err != nil {
		t.Fatalf("saveInterval: %v", err)
	}
	if err := saveTheme("mono"); err != nil {
		t.Fatalf("saveTheme: %v", err)
	}
	if name, seconds, _ := LoadConfig(); name != "mono" || seconds != 11 {
		t.Errorf("after saving a theme: (%q, %d), want (\"mono\", 11)", name, seconds)
	}
	if err := saveInterval(13); err != nil {
		t.Fatalf("second saveInterval: %v", err)
	}
	if name, seconds, _ := LoadConfig(); name != "mono" || seconds != 13 {
		t.Errorf("after saving an interval: (%q, %d), want (\"mono\", 13)", name, seconds)
	}
}

func TestConfigOutOfRangeIntervalFallsBack(t *testing.T) {
	for _, bad := range []string{"0", "-5", "99999"} {
		t.Run(bad, func(t *testing.T) {
			configHome(t)
			path := writeConfig(t, `{"theme":"nord","refreshIntervalSeconds":`+bad+`}`)
			name, seconds, warning := LoadConfig()
			if seconds != defaultRefreshSeconds {
				t.Errorf("seconds = %d, want the default %d", seconds, defaultRefreshSeconds)
			}
			if !strings.Contains(warning, path) || !strings.Contains(warning, bad) {
				t.Errorf("warning does not name the file and the value: %q", warning)
			}
			// the bad interval must not cost the user their theme
			if name != "nord" {
				t.Errorf("theme = %q, want nord", name)
			}
		})
	}
}

func TestConfigUnknownKeyWarnsAndStillLoads(t *testing.T) {
	configHome(t)
	path := writeConfig(t, `{"theme":"nord","refreshIntervalSeconds":9,"wibble":true}`)
	name, seconds, warning := LoadConfig()
	if name != "nord" || seconds != 9 {
		t.Errorf("got (%q, %d), want (\"nord\", 9)", name, seconds)
	}
	if !strings.Contains(warning, "wibble") || !strings.Contains(warning, path) {
		t.Errorf("warning = %q, want it to name the unknown key and the file", warning)
	}
}

func TestConfigEmptyFileIsUsable(t *testing.T) {
	configHome(t)
	path := writeConfig(t, "")
	name, seconds, warning := LoadConfig()
	if name != defaultThemeName || seconds != defaultRefreshSeconds {
		t.Errorf("got (%q, %d), want the defaults (%q, %d)",
			name, seconds, defaultThemeName, defaultRefreshSeconds)
	}
	if !strings.Contains(warning, path) {
		t.Errorf("warning does not name the file: %q", warning)
	}
}

// TestThemeSavedMsgDoesNotCrashTheModel covers the degrade-to-memory path: the
// switch stays applied and the failure only shows as a status line.
func TestThemeSavedMsgDoesNotCrashTheModel(t *testing.T) {
	configHome(t)
	m := press(fixture(t), "t")
	if m.theme.Name != "nord" {
		t.Fatalf("theme = %q, want nord", m.theme.Name)
	}
	next, cmd := m.Update(themeSavedMsg{err: os.ErrPermission})
	after := next.(Model)
	if cmd != nil {
		t.Error("themeSavedMsg produced a follow-up cmd")
	}
	if after.theme.Name != "nord" {
		t.Errorf("a failed write reverted the theme to %q", after.theme.Name)
	}
	if after.statusKind != statusError || !strings.Contains(after.status, "could not remember the theme") {
		t.Errorf("status = %q kind = %d", after.status, after.statusKind)
	}
}

// TestCycleThemePersists drives the switch end to end through the model and
// the store, which is as far as a test can go without a terminal.
func TestCycleThemePersists(t *testing.T) {
	configHome(t)
	next, cmd := fixture(t).cycleTheme(1)
	if cmd == nil {
		t.Fatal("a theme switch did not try to save")
	}
	if _, ok := cmd().(themeSavedMsg); !ok {
		t.Fatalf("a theme switch produced %T", cmd())
	}
	name, warning := LoadTheme()
	if name != next.(Model).theme.Name || warning != "" {
		t.Errorf("store holds (%q, %q), want %q", name, warning, next.(Model).theme.Name)
	}
}
