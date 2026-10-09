package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// maxSuggestions caps the reserved rows. The suggestion list is part of the
// normal layout flow, not an overlay, so every row it shows is a row the table
// gives up; six is as many as a short terminal can spare.
const maxSuggestions = 6

// fuzzyScore ranks name against pattern as a case-insensitive subsequence.
// Lower is better, and an unmatched pattern reports ok == false rather than a
// large score, so the caller never has to pick a cutoff.
//
// The shape of the score: a prefix match starts at 0 and anything else starts
// at 100, so "th" can never rank "end-duplicates" above "theme". After that,
// 10 per character skipped before the first match pushes mid-string hits down,
// the gaps between matched characters prefer a tight run, and the name length
// breaks ties towards the shorter command.
func fuzzyScore(pattern, name string) (int, bool) {
	p, n := strings.ToLower(pattern), strings.ToLower(name)
	if p == "" {
		return len(name), true
	}

	score := 100
	if strings.HasPrefix(n, p) {
		score = 0
	}

	at, first, prev := 0, -1, -1
	for _, want := range p {
		idx := strings.IndexRune(n[at:], want)
		if idx < 0 {
			return 0, false
		}
		idx += at
		if first < 0 {
			first = idx
		} else {
			score += idx - prev - 1
		}
		prev, at = idx, idx+len(string(want))
	}
	return score + 10*first + len(name), true
}

// matchCommands ranks the registry against what has been typed so far. An
// empty pattern is not a wildcard search, it is "nothing typed yet", so the
// whole registry comes back in registry order.
func matchCommands(input string) []command {
	pattern := strings.TrimSpace(input)
	if pattern == "" {
		out := make([]command, len(commandOrder))
		copy(out, commandOrder)
		return out
	}

	type scored struct {
		cmd   command
		score int
	}
	// built in registry order, then sorted stably, so an exact score tie is
	// resolved by the registry and the ordering is identical on every call
	var hits []scored
	for _, c := range commandOrder {
		if score, ok := fuzzyScore(pattern, c.name); ok {
			hits = append(hits, scored{cmd: c, score: score})
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score < hits[j].score })
	out := make([]command, len(hits))
	for i, h := range hits {
		out[i] = h.cmd
	}
	return out
}

// suggestWindow is the first entry to render so sel stays visible in a window
// of rows entries. The window follows the selection rather than paging, which
// is what the arrow keys read as in a list this short.
func suggestWindow(count, sel, rows int) int {
	if rows <= 0 || count <= rows {
		return 0
	}
	start := sel - rows/2
	if start+rows > count {
		start = count - rows
	}
	if start < 0 {
		start = 0
	}
	return start
}

// suggestColumns sizes the three suggestion columns from the registry, so the
// widest command name sets the gutter once instead of the description moving
// about as the match list changes.
func suggestColumns(w int) (nameW, argsW, descW int) {
	for _, c := range commandOrder {
		if n := runewidth.StringWidth(" /"+c.name) + 1; n > nameW {
			nameW = n
		}
		if n := runewidth.StringWidth(c.args) + 1; n > argsW {
			argsW = n
		}
	}
	if nameW > w-2 {
		nameW = w - 2
	}
	if nameW < 1 {
		nameW = 1
	}
	if argsW > w-nameW-1 {
		argsW = w - nameW - 1
	}
	if argsW < 0 {
		argsW = 0
	}
	return nameW, argsW, w - nameW - argsW
}

// suggestionLines renders exactly suggestionRows() lines, each exactly w
// columns wide. View counts on both: the rows are reserved in the layout
// budget, so one line too many or too few changes the height of the whole
// frame.
func (m Model) suggestionLines(w int) []string {
	rows := m.suggestionRows()
	if rows <= 0 {
		return nil
	}
	st := m.styles()
	th := m.activeTheme()
	matches := matchCommands(m.cmdInput)

	// the last row becomes the "more" indicator when the list does not fit;
	// with a single row available that row IS the indicator
	entries := rows
	truncated := len(matches) > rows
	if truncated {
		entries = rows - 1
	}
	nameW, argsW, descW := suggestColumns(w)
	start := suggestWindow(len(matches), m.cmdSel, entries)

	out := make([]string, 0, rows)
	for i := start; i < start+entries && i < len(matches); i++ {
		c := matches[i]
		name := lipgloss.NewStyle().Foreground(th.Text)
		dim := lipgloss.NewStyle().Foreground(th.Dim)
		if i == m.cmdSel {
			name = name.Background(th.SelBg).Bold(true)
			dim = dim.Background(th.SelBg)
		}
		out = append(out,
			name.Render(fit(" /"+c.name, nameW))+
				dim.Render(fit(c.args, argsW))+
				dim.Render(fit(c.desc, descW)))
	}
	if truncated {
		hidden := len(matches) - entries
		out = append(out, st.dim.Render(fit(fmt.Sprintf(" …%d more — keep typing", hidden), w)))
	}
	return out
}
