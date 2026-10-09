package ui

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"procdeck/internal/group"
)

const appVersion = "0.1.0-dev"

// command is one slash command. run takes the model and the already-trimmed
// argument text and returns the model it wants plus whatever side effect it
// needs; it may change the mode, which is why runCommand resets the mode
// before dispatching rather than after.
type command struct {
	name string
	args string
	desc string
	run  func(Model, string) (Model, tea.Cmd)
}

// commandOrder is the registry. It is an ordered slice because the suggestion
// list, the fuzzy tie-break and /help all need the same deterministic order.
//
// /help and /doctor are deliberately absent: they arrive with their panels, so
// no entry here ever has a handler that does nothing.
var commandOrder []command

// init fills the registry. It cannot be a plain var initialiser: a handler
// reaches rebuild → clampScroll → tableRows → suggestionRows → matchCommands,
// which reads commandOrder back, and Go rejects that as an initialization
// cycle even though nothing actually runs at init time.
func init() {
	commandOrder = []command{
		{"theme", "[name]", "switch the color theme, or cycle with no name", cmdTheme},
		{"themes", "", "list the available color themes", cmdThemes},
		{"filter", "<text>", "narrow the list to matching services", cmdFilter},
		{"all", "", "toggle showing every process, not just coding ones", cmdAll},
		{"clear", "", "clear the filter and the status line", cmdClear},
		{"refresh", "[seconds]", "show or set how often procdeck rescans", cmdRefresh},
		{"end", "[name]", "end a service, after a confirm", cmdEnd},
		{"restart", "[name]", "restart a service in a new window", cmdRestart},
		{"end-duplicates", "", "end every duplicate service at once", cmdEndDuplicates},
		{"open", "[name]", "open a service's link in a browser", cmdOpen},
		{"copy", "[name]", "copy a service's command line", cmdCopy},
		{"config", "", "show the config file path and the current settings", cmdConfig},
		{"version", "", "show the procdeck and Go versions", cmdVersion},
		{"quit", "", "quit procdeck", cmdQuit},
	}
}

func commandByName(name string) (command, bool) {
	for _, c := range commandOrder {
		if c.name == name {
			return c, true
		}
	}
	return command{}, false
}

// parseCommand splits a typed line into a command name and its argument text.
// It strips exactly ONE leading slash, so "//help" is the command "/help" and
// reports as unknown instead of quietly running help.
func parseCommand(line string) (name, arg string) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "/")
	if i := strings.IndexAny(line, " \t"); i >= 0 {
		name, arg = line[:i], strings.TrimSpace(line[i:])
	} else {
		name = line
	}
	return strings.ToLower(name), arg
}

// runCommand dispatches a typed line. The mode and the input are reset first,
// so a handler that wants another mode (the confirm prompt) can simply set it
// and a handler that does not leaves the user back in the list.
func runCommand(m Model, line string) (Model, tea.Cmd) {
	m.mode, m.cmdInput, m.cmdSel = modeList, "", 0
	m.clampScroll()

	name, arg := parseCommand(line)
	if name == "" {
		m.setStatus("no command typed — /help lists them", true)
		return m, nil
	}
	c, ok := commandByName(name)
	if !ok {
		m.setStatus(fmt.Sprintf("unknown command %q — /help lists them", name), true)
		return m, nil
	}
	return c.run(m, arg)
}

// resolveService turns a command argument into one service. An empty argument
// means the row under the cursor, which is what makes /end behave like x.
// A name is matched loosely because the user is typing, not pasting; an
// ambiguous match reports every candidate and acts on nothing, so a slash
// command can never end a service the user did not name.
func (m Model) resolveService(arg string) (group.Service, string) {
	if arg == "" {
		svc, ok := m.current()
		if !ok {
			return group.Service{}, "no service selected"
		}
		return svc, ""
	}

	needle := strings.ToLower(arg)
	var hits []group.Service
	for _, svc := range m.visible {
		if strings.Contains(strings.ToLower(svc.Name), needle) ||
			strings.Contains(strings.ToLower(svc.Project), needle) {
			hits = append(hits, svc)
		}
	}
	switch len(hits) {
	case 0:
		return group.Service{}, fmt.Sprintf("nothing visible matches %q", arg)
	case 1:
		return hits[0], ""
	}
	names := make([]string, len(hits))
	for i, svc := range hits {
		names[i] = svc.Name
	}
	return group.Service{}, fmt.Sprintf("%d services match %q: %s — name one of them",
		len(hits), arg, strings.Join(names, ", "))
}

// suggestOpen reports whether the suggestion list is showing. It closes as
// soon as the input holds a space: up to that point the user is still naming a
// command, after it they are typing arguments and enter must take the literal
// text.
func (m Model) suggestOpen() bool {
	return m.mode == modeCommand && !strings.Contains(m.cmdInput, " ")
}

func (m Model) selectedSuggestion() (command, bool) {
	if !m.suggestOpen() {
		return command{}, false
	}
	matches := matchCommands(m.cmdInput)
	if m.cmdSel < 0 || m.cmdSel >= len(matches) {
		return command{}, false
	}
	return matches[m.cmdSel], true
}

func (m *Model) clampCmdSel() {
	if n := len(matchCommands(m.cmdInput)); m.cmdSel >= n {
		m.cmdSel = n - 1
	}
	if m.cmdSel < 0 {
		m.cmdSel = 0
	}
}

// updateCommand is the key handling for the command input. Every branch ends
// in clampScroll because the suggestion rows are part of the layout budget:
// one more or one fewer match changes tableRows, and the cursor has to stay
// inside the table that is left.
func (m Model) updateCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m.interrupt()
	case tea.KeyEsc:
		m.mode, m.cmdInput, m.cmdSel = modeList, "", 0
	case tea.KeyBackspace:
		r := []rune(m.cmdInput)
		if len(r) == 0 {
			// backspacing past the / leaves command mode the way it was entered
			m.mode, m.cmdSel = modeList, 0
		} else {
			m.cmdInput, m.cmdSel = string(r[:len(r)-1]), 0
		}
	case tea.KeyTab:
		if c, ok := m.selectedSuggestion(); ok {
			m.cmdInput, m.cmdSel = c.name, 0
			if c.args != "" {
				// a trailing space both reads as "your turn" and closes the
				// suggestion list, so enter will take what was typed
				m.cmdInput += " "
			}
		}
	case tea.KeyUp:
		m.cmdSel--
	case tea.KeyDown:
		m.cmdSel++
	case tea.KeyEnter:
		line := m.cmdInput
		if c, ok := m.selectedSuggestion(); ok {
			line = c.name
		}
		next, cmd := runCommand(m, line)
		return next, cmd
	case tea.KeySpace:
		m.cmdInput, m.cmdSel = m.cmdInput+" ", 0
	case tea.KeyRunes:
		m.cmdInput, m.cmdSel = m.cmdInput+string(msg.Runes), 0
	}
	m.clampCmdSel()
	m.clampScroll()
	return m, nil
}

func cmdTheme(m Model, arg string) (Model, tea.Cmd) {
	if arg == "" {
		next, cmd := m.cycleTheme(1)
		return next.(Model), cmd
	}
	if t, ok := themeByName(strings.ToLower(arg)); ok {
		m.theme = t
		m.setStatus("theme: "+t.Name, false)
		return m, saveThemeCmd(t.Name)
	}
	m.setStatus(fmt.Sprintf("unknown theme %q — try one of: %s",
		arg, strings.Join(ThemeNames(), ", ")), true)
	return m, nil
}

func cmdThemes(m Model, _ string) (Model, tea.Cmd) {
	m.setStatus(fmt.Sprintf("themes: %s · active %s",
		strings.Join(ThemeNames(), ", "), m.activeTheme().Name), false)
	return m, nil
}

func cmdFilter(m Model, arg string) (Model, tea.Cmd) {
	if arg == "" {
		m.setStatus("usage: /filter <text> — /clear removes an active filter", true)
		return m, nil
	}
	m.filter = arg
	m.rebuild()
	return m, nil
}

func cmdAll(m Model, _ string) (Model, tea.Cmd) {
	m.showAll = !m.showAll
	m.rebuild()
	return m, nil
}

func cmdClear(m Model, _ string) (Model, tea.Cmd) {
	m.filter = ""
	m.status, m.statusKind = "", statusInfo
	m.rebuild()
	return m, nil
}

// refreshRange is shared by every /refresh error so each one states the floor;
// a rejected interval that does not say what is allowed is a dead end.
func refreshRange() string {
	return fmt.Sprintf("%d second to %d seconds", minRefreshSeconds, maxRefreshSeconds)
}

func cmdRefresh(m Model, arg string) (Model, tea.Cmd) {
	if arg == "" {
		m.setStatus(fmt.Sprintf("scanning every %ds", int(m.refreshInterval().Seconds())), false)
		return m, nil
	}
	n, err := strconv.Atoi(arg)
	if err != nil {
		m.setStatus(fmt.Sprintf("%q is not a number — /refresh takes %s", arg, refreshRange()), true)
		return m, nil
	}
	switch {
	case n < minRefreshSeconds:
		m.setStatus(fmt.Sprintf("%ds is too fast — the floor is %s", n, refreshRange()), true)
		return m, nil
	case n > maxRefreshSeconds:
		m.setStatus(fmt.Sprintf("%ds is too slow — /refresh takes %s", n, refreshRange()), true)
		return m, nil
	}
	m.refreshSeconds = n
	m.setStatus(fmt.Sprintf("scanning every %ds from the next scan", n), false)
	return m, saveIntervalCmd(n)
}

// cmdEnd, cmdRestart and cmdEndDuplicates deliberately go no further than
// m.ask: the prompt text and the command are the same ones the x, r and D keys
// build, so there is exactly one confirm path and a slash command cannot
// bypass it.
func cmdEnd(m Model, arg string) (Model, tea.Cmd) {
	svc, problem := m.resolveService(arg)
	if problem != "" {
		m.setStatus(problem, true)
		return m, nil
	}
	m.ask(fmt.Sprintf("End %s?", describe(svc)), endCmd(svc))
	return m, nil
}

func cmdRestart(m Model, arg string) (Model, tea.Cmd) {
	svc, problem := m.resolveService(arg)
	if problem != "" {
		m.setStatus(problem, true)
		return m, nil
	}
	m.ask(fmt.Sprintf("Restart %s?", describe(svc)), restartCmd(svc))
	return m, nil
}

func cmdEndDuplicates(m Model, _ string) (Model, tea.Cmd) {
	if n := countDuplicates(m.services); n > 0 {
		m.ask(fmt.Sprintf("End all %d duplicates?", n), endDuplicatesCmd(m.services))
	} else {
		m.setStatus("no duplicates", false)
	}
	return m, nil
}

func cmdOpen(m Model, arg string) (Model, tea.Cmd) {
	svc, problem := m.resolveService(arg)
	if problem != "" {
		m.setStatus(problem, true)
		return m, nil
	}
	return m, openCmd(svc)
}

func cmdCopy(m Model, arg string) (Model, tea.Cmd) {
	svc, problem := m.resolveService(arg)
	if problem != "" {
		m.setStatus(problem, true)
		return m, nil
	}
	return m, copyCmd(svc)
}

// cmdConfig reports the path plus the settings that are live right now, which
// is not the same as what is on disk: an unsaved /refresh shows here too.
func cmdConfig(m Model, _ string) (Model, tea.Cmd) {
	path, err := configPath()
	if err != nil {
		path = "unavailable (" + err.Error() + ")"
	}
	m.setStatus(fmt.Sprintf("config: %s · theme %s · scan %ds",
		path, m.activeTheme().Name, int(m.refreshInterval().Seconds())), false)
	return m, nil
}

func cmdVersion(m Model, _ string) (Model, tea.Cmd) {
	m.setStatus(fmt.Sprintf("procdeck %s · %s · %s/%s",
		appVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH), false)
	return m, nil
}

func cmdQuit(m Model, _ string) (Model, tea.Cmd) {
	return m, tea.Quit
}
