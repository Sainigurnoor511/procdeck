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

var (
	colAccent = lipgloss.AdaptiveColor{Dark: "#D97757", Light: "#B2552F"}
	colText   = lipgloss.AdaptiveColor{Dark: "#E8E4DF", Light: "#1C1A17"}
	colDim    = lipgloss.AdaptiveColor{Dark: "#8A827A", Light: "#6B635B"}
	colBorder = lipgloss.AdaptiveColor{Dark: "#45403A", Light: "#D6CFC7"}
	colGreen  = lipgloss.AdaptiveColor{Dark: "#7FA86A", Light: "#3F6B2E"}
	colAmber  = lipgloss.AdaptiveColor{Dark: "#D3A05A", Light: "#8A5E14"}
	colRed    = lipgloss.AdaptiveColor{Dark: "#C96A62", Light: "#A3362E"}
	colSelBg  = lipgloss.AdaptiveColor{Dark: "#2E2A26", Light: "#EFE8E0"}

	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleBorder  = lipgloss.NewStyle().Foreground(colBorder)
	styleHead    = lipgloss.NewStyle().Bold(true).Foreground(colDim)
	styleDim     = lipgloss.NewStyle().Foreground(colDim)
	styleText    = lipgloss.NewStyle().Foreground(colText)
	styleHintKey = lipgloss.NewStyle().Bold(true).Foreground(colDim)
	styleAccent  = lipgloss.NewStyle().Foreground(colAccent)
	styleLabel   = lipgloss.NewStyle().Foreground(colAccent)
	styleOK      = lipgloss.NewStyle().Foreground(colGreen)
	styleWarn    = lipgloss.NewStyle().Foreground(colAmber).Bold(true)
	styleErr     = lipgloss.NewStyle().Foreground(colRed)

	// every box border rune comes from here, nothing is hand-written
	rounded = lipgloss.RoundedBorder()
)

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

// chrome counts every line View renders apart from the table's data rows.
func (m Model) chrome() int {
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
		rows = append(rows, styleDim.Render(fit("  "+m.emptyText(), inner)))
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
	out = append(out, box("", foot, rows, inner)...)

	if m.showDetail() {
		out = append(out, m.detail(inner)...)
	}
	if m.showStatus() {
		out = append(out, m.statusLine(w))
	}
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
	left := styleTitle.Render(" ✻ procdeck")
	dups := countDuplicates(m.services)
	right := styleDim.Render(plural(len(m.services), "service"))
	if dups > 0 {
		right += styleDim.Render(" · ") + styleWarn.Render("▲ "+plural(dups, "duplicate"))
	}
	if m.showAll {
		right += styleDim.Render(" · all processes")
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
	return styleHead.Render(s)
}

func (m Model) row(svc group.Service, c columns, selected bool) string {
	base := lipgloss.NewStyle().Foreground(colText)
	dot := lipgloss.NewStyle().Foreground(colGreen)
	glyph := "●"
	switch {
	case svc.Restricted():
		base = base.Foreground(colDim)
		dot, glyph = dot.Foreground(colDim), "○"
	case svc.Duplicate:
		base = base.Foreground(colAmber)
		dot, glyph = dot.Foreground(colAmber), "▲"
	}
	muted := lipgloss.NewStyle().Foreground(colDim)
	link := lipgloss.NewStyle().Foreground(colAccent).Underline(true)
	if selected {
		base = base.Background(colSelBg).Bold(true)
		dot = dot.Background(colSelBg)
		muted = muted.Background(colSelBg)
		link = link.Background(colSelBg)
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
	svc, ok := m.current()
	if !ok {
		blank := strings.Repeat(" ", inner)
		return box("details", "", []string{blank, blank, blank, blank}, inner)
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
		note = styleWarn.Render(fit(" ▲ another copy of this service is already running in the same folder", inner))
	case svc.Protected:
		note = styleDim.Render(fit(" procdeck is running inside this process tree — end and restart are disabled", inner))
	case svc.Restricted():
		note = styleDim.Render(fit(" no access to this process — run procdeck as administrator to manage it", inner))
	default:
		note = strings.Repeat(" ", inner)
	}

	field := func(label, value string) string {
		return styleLabel.Render(" "+label+" ") + styleText.Render(fit(value, inner-len(label)-2))
	}
	lines := []string{
		styleText.Render(fit(" "+strings.Join(facts, "  ·  "), inner)),
		field("cmd", svc.Root.Cmdline),
		field("cwd", svc.Root.Cwd),
		note,
	}
	return box(svc.Name, "", lines, inner)
}

// inputBox is always rendered: it is the prompt, the filter editor and the
// confirm prompt in one always-visible slot.
func (m Model) inputBox(w int) string {
	inner := w - 2
	border, fg := colBorder, colText
	content := "> " + m.filter + "   (esc clears)"
	switch {
	case m.mode == modeConfirm:
		border, fg = colAmber, colAmber
		// the answer keys must survive truncation, so the prompt yields to them
		const glyph, answer = "▲ ", "   y / n"
		budget := inner - 1 - runewidth.StringWidth(glyph) - runewidth.StringWidth(answer)
		prompt := "…"
		if budget > 0 {
			prompt = truncate(m.prompt, budget)
		}
		content = glyph + prompt + answer
	case m.mode == modeFilter:
		border = colAccent
		content = "> " + m.filter + "▌"
	case m.filter == "":
		fg = colDim
		content = "> press / to filter"
	}
	// Width is the content width, so the rendered block is inner+2 == w columns.
	return lipgloss.NewStyle().
		Border(rounded).
		BorderForeground(border).
		Foreground(fg).
		Width(inner).
		Render(fit(content, inner))
}

func (m Model) keys(w int) string {
	pairs := [][2]string{
		{"↑↓", "move"}, {"/", "filter"}, {"x", "end"}, {"r", "restart"},
		{"q", "quit"}, {"D", "dups"}, {"o", "open"}, {"c", "copy"}, {"a", "all"},
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
			b.WriteString(styleDim.Render(" · "))
		}
		b.WriteString(styleHintKey.Render(p[0]) + " " + styleDim.Render(p[1]))
		used += need
	}
	return b.String()
}

// busyLine is the spinner: one accent frame glyph plus a dim label and the
// seconds the work has been running for.
func (m Model) busyLine() string {
	glyph := spinFrames[m.frame%len(spinFrames)]
	rest := fmt.Sprintf(" %s… (%ds)", m.busy, int(m.busyFor.Seconds()))
	return styleAccent.Render(glyph) + styleDim.Render(rest)
}

func (m Model) statusLine(w int) string {
	if m.busy != "" && m.loaded {
		return padTo(" "+m.busyLine(), w)
	}
	text := m.status
	style := styleOK
	switch m.statusKind {
	case statusError:
		style = styleErr
	case statusHint:
		style = styleDim
	}
	if text == "" && m.filter != "" {
		text, style = "filter: "+m.filter+"  (esc clears)", styleDim
	}
	return style.Render(fit(" "+text, w))
}

func box(title, foot string, lines []string, inner int) []string {
	edge := func(left, h, label, right string) string {
		if label != "" {
			label = " " + truncate(label, inner-4) + " "
		}
		fill := inner - runewidth.StringWidth(label) - 1
		if fill < 0 {
			fill = 0
		}
		return styleBorder.Render(left+h) + styleDim.Render(label) + styleBorder.Render(strings.Repeat(h, fill)+right)
	}
	left, right := styleBorder.Render(rounded.Left), styleBorder.Render(rounded.Right)
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
