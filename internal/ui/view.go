package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"procdeck/internal/group"
)

// every box border rune comes from here, nothing is hand-written
var rounded = lipgloss.RoundedBorder()

// activeTheme never returns a zero Theme, so a hand-built Model{} renders in
// the default theme instead of with empty color strings.
func (m Model) activeTheme() Theme {
	if m.theme.Name == "" {
		return themeOrder[0]
	}
	return m.theme
}

func (m Model) styles() styles { return newStyles(m.activeTheme()) }

type columns struct {
	name, project, port, link, cpu, mem, up int
}

const (
	minWidth     = 60
	detailLines  = 6
	detailCutoff = 18
	hintCutoff   = 12
	statusCutoff = 10
	inputLines   = 3 // rounded border + 1 content line
	tableChrome  = 3 // top border + head row + bottom border
)

func (m Model) viewWidth() int {
	if m.width < minWidth {
		return minWidth
	}
	return m.width
}

func (m Model) showDetail() bool { return m.height >= detailCutoff }
func (m Model) showHint() bool   { return m.height >= hintCutoff }
func (m Model) showStatus() bool { return m.height >= statusCutoff }

// baseChrome counts every line View renders apart from the table's data rows
// and the suggestion rows.
func (m Model) baseChrome() int {
	n := 1 + tableChrome + inputLines // header + table frame + input box
	if m.showDetail() {
		n += detailLines
	}
	if m.showStatus() {
		n++
	}
	if m.showHint() {
		n++
	}
	return n
}

// suggestionRows is how many lines the command suggestions occupy. Bubble Tea
// v1 has no compositor, so the list is reserved rows in the normal layout flow
// rather than a floating overlay: every row it takes is a row the table gives
// up.
//
// It is computed from baseChrome, never from tableRows, because tableRows
// subtracts this; and it is clamped to leave one line behind so the table
// always keeps at least one row.
func (m Model) suggestionRows() int {
	if !m.suggestOpen() {
		return 0
	}
	n := len(matchCommands(m.cmdInput))
	if n > maxSuggestions {
		n = maxSuggestions
	}
	if room := m.height - m.baseChrome() - 1; n > room {
		n = room
	}
	if n < 0 {
		n = 0
	}
	return n
}

// chrome counts every line View renders apart from the table's data rows.
func (m Model) chrome() int {
	return m.baseChrome() + m.suggestionRows()
}

func (m Model) tableRows() int {
	rows := m.height - m.chrome()
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m Model) layout() columns {
	inner := m.viewWidth() - 2
	c := columns{port: 9, cpu: 7, mem: 8}
	if inner >= 80 {
		c.up = 8
	}
	if inner >= 104 {
		c.link = 25
	}
	rest := inner - 3 - c.port - c.cpu - c.mem - c.up - c.link
	c.name = rest * 55 / 100
	c.project = rest - c.name
	return c
}

func (m Model) View() string {
	s := m.styles()
	w := m.viewWidth()
	inner := w - 2
	cols := m.layout()

	var out []string
	out = append(out, m.header(w))

	rows := []string{m.headRow(cols)}
	limit := m.tableRows()
	switch {
	case !m.loaded:
		rows = append(rows, padTo(" "+m.busyLine(), inner))
	case len(m.visible) == 0:
		rows = append(rows, s.dim.Render(fit("  "+m.emptyText(), inner)))
	}
	end := m.offset + limit
	if end > len(m.visible) {
		end = len(m.visible)
	}
	for i := m.offset; i < end; i++ {
		rows = append(rows, m.row(m.visible[i], cols, i == m.cursor))
	}
	for len(rows) < limit+1 {
		rows = append(rows, strings.Repeat(" ", inner))
	}
	foot := ""
	if len(m.visible) > limit {
		foot = fmt.Sprintf("%d–%d of %d", m.offset+1, end, len(m.visible))
	}
	out = append(out, m.box("", foot, rows, inner)...)

	if m.showDetail() {
		out = append(out, m.detail(inner)...)
	}
	if m.showStatus() {
		out = append(out, m.statusLine(w))
	}
	out = append(out, m.suggestionLines(w)...)
	out = append(out, strings.Split(m.inputBox(w), "\n")...)
	if m.showHint() {
		out = append(out, m.keys(w))
	}
	return strings.Join(out, "\n")
}

func (m Model) emptyText() string {
	if m.filter != "" {
		return "nothing matches “" + m.filter + "” — esc clears the filter"
	}
	return "no coding processes running — press a to show everything"
}

func (m Model) header(w int) string {
	s := m.styles()
	left := s.title.Render(" ✻ procdeck")
	dups := countDuplicates(m.services)
	right := s.dim.Render(plural(len(m.services), "service"))
	if dups > 0 {
		right += s.dim.Render(" · ") + s.warn.Render("▲ "+plural(dups, "duplicate"))
	}
	if m.showAll {
		right += s.dim.Render(" · all processes")
	}
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m Model) headRow(c columns) string {
	s := "   " + fit("NAME", c.name) + fit("PROJECT", c.project) + fit("PORT", c.port)
	if c.link > 0 {
		s += fit("LINK", c.link)
	}
	s += fitRight("CPU", c.cpu) + fitRight("MEM", c.mem)
	if c.up > 0 {
		s += fitRight("UP", c.up)
	}
	return m.styles().head.Render(s)
}

func (m Model) row(svc group.Service, c columns, selected bool) string {
	th := m.activeTheme()
	base := lipgloss.NewStyle().Foreground(th.Text)
	dot := lipgloss.NewStyle().Foreground(th.Green)
	glyph := "●"
	switch {
	case svc.Restricted():
		base = base.Foreground(th.Dim)
		dot, glyph = dot.Foreground(th.Dim), "○"
	case svc.Duplicate:
		base = base.Foreground(th.Amber)
		dot, glyph = dot.Foreground(th.Amber), "▲"
		if th.DupBold {
			// a palette with no hue to spare pays for the duplicate
			// distinction in weight instead; bold is not a width
			base, dot = base.Bold(true), dot.Bold(true)
		}
	}
	muted := lipgloss.NewStyle().Foreground(th.Dim)
	link := lipgloss.NewStyle().Foreground(th.Accent).Underline(true)
	if selected {
		base = base.Background(th.SelBg).Bold(true)
		dot = dot.Background(th.SelBg)
		muted = muted.Background(th.SelBg)
		link = link.Background(th.SelBg)
	}

	name := svc.Name
	if svc.Duplicate {
		name += " (duplicate)"
	}
	s := dot.Render(" "+glyph+" ") +
		base.Render(fit(name, c.name)) +
		muted.Render(fit(svc.Project, c.project)) +
		base.Render(fit(portText(svc.Ports), c.port))
	if c.link > 0 {
		url := truncate(svc.Link, c.link-1)
		pad := strings.Repeat(" ", c.link-runewidth.StringWidth(url))
		if url == "" {
			s += muted.Render(pad)
		} else {
			s += hyperlink(svc.Link, link.Render(url)) + muted.Render(pad)
		}
	}
	s += base.Render(fitRight(cpuText(svc.CPU), c.cpu)) + base.Render(fitRight(memText(svc.Mem), c.mem))
	if c.up > 0 {
		s += muted.Render(fitRight(m.upText(svc), c.up))
	}
	return s
}

func (m Model) detail(inner int) []string {
	s := m.styles()
	svc, ok := m.current()
	if !ok {
		blank := strings.Repeat(" ", inner)
		return m.box("details", "", []string{blank, blank, blank, blank}, inner)
	}
	facts := []string{"pid " + strconv.Itoa(int(svc.Root.PID))}
	if svc.Variant != "" {
		facts = append(facts, svc.Variant+" python")
	}
	facts = append(facts, plural(len(svc.Procs), "process"), "up "+m.upText(svc))
	if len(svc.Ports) > 0 {
		ports := make([]string, len(svc.Ports))
		for i, p := range svc.Ports {
			ports[i] = strconv.Itoa(p)
		}
		facts = append(facts, "ports "+strings.Join(ports, ", "))
	}

	note := ""
	switch {
	case svc.Duplicate:
		note = s.warn.Render(fit(" ▲ another copy of this service is already running in the same folder", inner))
	case svc.Protected:
		note = s.dim.Render(fit(" procdeck is running inside this process tree — end and restart are disabled", inner))
	case svc.Restricted():
		note = s.dim.Render(fit(" no access to this process — run procdeck as administrator to manage it", inner))
	default:
		note = strings.Repeat(" ", inner)
	}

	field := func(label, value string) string {
		return s.label.Render(" "+label+" ") + s.text.Render(fit(value, inner-len(label)-2))
	}
	lines := []string{
		s.text.Render(fit(" "+strings.Join(facts, "  ·  "), inner)),
		field("cmd", svc.Root.Cmdline),
		field("cwd", svc.Root.Cwd),
		note,
	}
	return m.box(svc.Name, "", lines, inner)
}

// inputBox is always rendered: it is the prompt, the filter editor and the
// confirm prompt in one always-visible slot.
func (m Model) inputBox(w int) string {
	th := m.activeTheme()
	inner := w - 2
	border, fg := th.Border, th.Text
	content := "> " + m.filter + "   (esc clears)"
	switch {
	case m.mode == modeConfirm:
		border, fg = th.Amber, th.Amber
		// the answer keys must survive truncation, so the prompt yields to them
		const glyph, answer = "▲ ", "   y / n"
		budget := inner - 1 - runewidth.StringWidth(glyph) - runewidth.StringWidth(answer)
		prompt := "…"
		if budget > 0 {
			prompt = truncate(m.prompt, budget)
		}
		content = glyph + prompt + answer
	case m.mode == modeCommand:
		border = th.Accent
		content = "> /" + m.cmdInput + "▌"
	case m.mode == modeFilter:
		border = th.Accent
		content = "> " + m.filter + "▌"
	case m.filter == "":
		fg = th.Dim
		content = "> press / for commands, f to filter"
	}
	// Width is the content width, so the rendered block is inner+2 == w columns.
	return lipgloss.NewStyle().
		Border(rounded).
		BorderForeground(border).
		Foreground(fg).
		Width(inner).
		Render(fit(content, inner))
}

// keys is the footer hint line. The pairs come from keymap in table order, so
// the footer and /help can never drift apart, and the table is ordered so the
// bindings that matter most survive the narrowest widths.
func (m Model) keys(w int) string {
	st := m.styles()
	var pairs [][2]string
	for _, b := range keymap {
		if b.hint[0] != "" {
			pairs = append(pairs, b.hint)
		}
	}
	var b strings.Builder
	used := 0
	for i, p := range pairs {
		// leading space for the first pair, " · " separator for the rest
		need := 1 + runewidth.StringWidth(p[0]) + 1 + runewidth.StringWidth(p[1])
		if i > 0 {
			need += 2
		}
		if used+need > w {
			break
		}
		if i == 0 {
			b.WriteString(" ")
		} else {
			b.WriteString(st.dim.Render(" · "))
		}
		b.WriteString(st.hintKey.Render(p[0]) + " " + st.dim.Render(p[1]))
		used += need
	}
	return b.String()
}

// busyLine is the spinner: one accent frame glyph plus a dim label and the
// seconds the work has been running for.
func (m Model) busyLine() string {
	s := m.styles()
	glyph := spinFrames[m.frame%len(spinFrames)]
	rest := fmt.Sprintf(" %s… (%ds)", m.busy, int(m.busyFor.Seconds()))
	return s.accent.Render(glyph) + s.dim.Render(rest)
}

func (m Model) statusLine(w int) string {
	if m.busy != "" && m.loaded {
		return padTo(" "+m.busyLine(), w)
	}
	s := m.styles()
	text := m.status
	style := s.ok
	switch m.statusKind {
	case statusError:
		style = s.err
	case statusHint:
		style = s.dim
	}
	if text == "" && m.filter != "" {
		text, style = "filter: "+m.filter+"  (esc clears)", s.dim
	}
	return style.Render(fit(" "+text, w))
}

func (m Model) box(title, foot string, lines []string, inner int) []string {
	s := m.styles()
	edge := func(left, h, label, right string) string {
		if label != "" {
			label = " " + truncate(label, inner-4) + " "
		}
		fill := inner - runewidth.StringWidth(label) - 1
		if fill < 0 {
			fill = 0
		}
		return s.border.Render(left+h) + s.dim.Render(label) + s.border.Render(strings.Repeat(h, fill)+right)
	}
	left, right := s.border.Render(rounded.Left), s.border.Render(rounded.Right)
	out := []string{edge(rounded.TopLeft, rounded.Top, title, rounded.TopRight)}
	for _, l := range lines {
		out = append(out, left+l+right)
	}
	return append(out, edge(rounded.BottomLeft, rounded.Bottom, foot, rounded.BottomRight))
}

func hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if runewidth.StringWidth(s) <= w {
		return s
	}
	return runewidth.Truncate(s, w, "…")
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = truncate(s, w-1)
	return s + strings.Repeat(" ", w-runewidth.StringWidth(s))
}

// padTo pads an already-styled string to w columns. fit cannot be used here:
// runewidth would count the ANSI escapes, lipgloss.Width does not.
func padTo(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func fitRight(s string, w int) string {
	if w <= 0 {
		return ""
	}
	s = truncate(s, w-1)
	return strings.Repeat(" ", w-1-runewidth.StringWidth(s)) + s + " "
}

func portText(ports []int) string {
	switch len(ports) {
	case 0:
		return "—"
	case 1:
		return strconv.Itoa(ports[0])
	}
	return fmt.Sprintf("%d +%d", ports[0], len(ports)-1)
}

func cpuText(cpu float64) string {
	return strconv.FormatFloat(cpu, 'f', 1, 64) + "%"
}

func memText(bytes uint64) string {
	mb := float64(bytes) / (1 << 20)
	if mb >= 1024 {
		return strconv.FormatFloat(mb/1024, 'f', 1, 64) + "G"
	}
	return strconv.FormatFloat(mb, 'f', 0, 64) + "M"
}

func (m Model) upText(svc group.Service) string {
	if svc.Root.Created.UnixMilli() <= 0 {
		return "—"
	}
	return uptime(m.snap.At.Sub(svc.Root.Created))
}

func uptime(d time.Duration) string {
	switch {
	case d < 0:
		return "0s"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	if strings.HasSuffix(noun, "s") {
		return fmt.Sprintf("%d %ses", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
