package ui

import "github.com/charmbracelet/lipgloss"

// Theme is the whole palette the renderer reads. Every field stays an
// AdaptiveColor so a theme still follows the terminal's light or dark
// background; a theme with one intended tone sets both halves to related
// values rather than dropping the adaptation.
type Theme struct {
	Name   string
	Accent lipgloss.AdaptiveColor
	Text   lipgloss.AdaptiveColor
	Dim    lipgloss.AdaptiveColor
	Border lipgloss.AdaptiveColor
	Green  lipgloss.AdaptiveColor
	Amber  lipgloss.AdaptiveColor
	Red    lipgloss.AdaptiveColor
	SelBg  lipgloss.AdaptiveColor

	// DupBold is the DupBold contingency the themes review pre-decided: when a
	// palette has no hue to spare, the duplicate row carries weight as well as
	// lightness. Only mono sets it, and only because re-spreading its ramp to
	// free Red from Accent cost the Text/Amber gap most of its brightness.
	DupBold bool
}

// styles is the derived set every render method works from. Rebuilding it per
// method keeps the active theme in one place on the Model; row() already
// constructs styles per frame, so this is the existing cost profile.
type styles struct {
	title, border, head, dim, text, hintKey, accent, label, ok, warn, err lipgloss.Style
}

func newStyles(t Theme) styles {
	return styles{
		title:   lipgloss.NewStyle().Bold(true).Foreground(t.Accent),
		border:  lipgloss.NewStyle().Foreground(t.Border),
		head:    lipgloss.NewStyle().Bold(true).Foreground(t.Dim),
		dim:     lipgloss.NewStyle().Foreground(t.Dim),
		text:    lipgloss.NewStyle().Foreground(t.Text),
		hintKey: lipgloss.NewStyle().Bold(true).Foreground(t.Dim),
		accent:  lipgloss.NewStyle().Foreground(t.Accent),
		label:   lipgloss.NewStyle().Foreground(t.Accent),
		ok:      lipgloss.NewStyle().Foreground(t.Green),
		warn:    lipgloss.NewStyle().Foreground(t.Amber).Bold(true),
		err:     lipgloss.NewStyle().Foreground(t.Red),
	}
}

const defaultThemeName = "claude"

// themeOrder is the registry: an ordered slice, because the cycle keys, the
// -themes listing and the default all need the same deterministic order.
// themeOrder[0] is the default and must keep rendering byte-identically.
var themeOrder = []Theme{
	// claude: procdeck's original palette, warm terracotta on near-black.
	{
		Name:   "claude",
		Accent: lipgloss.AdaptiveColor{Dark: "#D97757", Light: "#B2552F"},
		Text:   lipgloss.AdaptiveColor{Dark: "#E8E4DF", Light: "#1C1A17"},
		Dim:    lipgloss.AdaptiveColor{Dark: "#8A827A", Light: "#6B635B"},
		Border: lipgloss.AdaptiveColor{Dark: "#45403A", Light: "#D6CFC7"},
		Green:  lipgloss.AdaptiveColor{Dark: "#7FA86A", Light: "#3F6B2E"},
		Amber:  lipgloss.AdaptiveColor{Dark: "#D3A05A", Light: "#8A5E14"},
		Red:    lipgloss.AdaptiveColor{Dark: "#C96A62", Light: "#A3362E"},
		SelBg:  lipgloss.AdaptiveColor{Dark: "#2E2A26", Light: "#EFE8E0"},
	},
	// nord: cool blue-grey. The Aurora, Frost and Snow Storm values are Nord's
	// own (https://www.nordtheme.com/docs/colors-and-palettes/).
	//
	// Two values are deliberately NOT straight from the palette:
	//   - Dim is a lightened nord3. Nord's own comment grey (#4C566A, or the
	//     #616E88 many ports use) only reaches 2.8:1 on a dark terminal and
	//     1.9:1 against the selected row, which leaves the project column
	//     unreadable. #7B88A1 keeps the hue and lands where the default theme
	//     sits (4.2:1 / 3.4:1).
	//   - SelBg is nord0, Nord's background, not nord1. nord1 lifts the
	//     selected row further off a dark terminal than any other theme does.
	//
	// Nord ships dark-only, so the whole light half is DERIVED, not upstream:
	// the Aurora hues are darkened to stay legible on a light background and
	// Polar Night / Snow Storm supply text and surfaces. Do not "correct" it
	// against an official light Nord — there is none.
	{
		Name:   "nord",
		Accent: lipgloss.AdaptiveColor{Dark: "#88C0D0", Light: "#5E81AC"},
		Text:   lipgloss.AdaptiveColor{Dark: "#ECEFF4", Light: "#2E3440"},
		Dim:    lipgloss.AdaptiveColor{Dark: "#7B88A1", Light: "#4C566A"},
		Border: lipgloss.AdaptiveColor{Dark: "#434C5E", Light: "#D8DEE9"},
		Green:  lipgloss.AdaptiveColor{Dark: "#A3BE8C", Light: "#4F6B3C"},
		Amber:  lipgloss.AdaptiveColor{Dark: "#EBCB8B", Light: "#8A6D1F"},
		Red:    lipgloss.AdaptiveColor{Dark: "#BF616A", Light: "#99323C"},
		SelBg:  lipgloss.AdaptiveColor{Dark: "#2E3440", Light: "#E5E9F0"},
	},
	// gruvbox: warm retro. Dark is gruvbox dark with the bright accents, light
	// is gruvbox light with the faded ones (https://github.com/morhetz/gruvbox).
	// The light selection is light0_soft rather than light1: light1 is a loud
	// band on a white terminal and pulls the faded yellow down to 3.3:1.
	{
		Name:   "gruvbox",
		Accent: lipgloss.AdaptiveColor{Dark: "#FE8019", Light: "#AF3A03"},
		Text:   lipgloss.AdaptiveColor{Dark: "#EBDBB2", Light: "#3C3836"},
		Dim:    lipgloss.AdaptiveColor{Dark: "#928374", Light: "#7C6F64"},
		Border: lipgloss.AdaptiveColor{Dark: "#504945", Light: "#D5C4A1"},
		Green:  lipgloss.AdaptiveColor{Dark: "#B8BB26", Light: "#79740E"},
		Amber:  lipgloss.AdaptiveColor{Dark: "#FABD2F", Light: "#B57614"},
		Red:    lipgloss.AdaptiveColor{Dark: "#FB4934", Light: "#9D0006"},
		SelBg:  lipgloss.AdaptiveColor{Dark: "#3C3836", Light: "#F2E5BC"},
	},
	// tokyonight: purple-blue. Dark is Night, light is Day, both from the
	// project's terminal extras (https://github.com/folke/tokyonight.nvim).
	//
	// Dim and Border take the palette's dark5 / fg_gutter rather than its
	// comment and bg_highlight: those two are meant to be backgrounds, and as
	// line colors they fall to 2.3:1 and 1.2:1, which makes the box borders
	// all but invisible. The light selection is Day's bg_highlight for the
	// same reason — bg_visual crushed the selected row's text to 3.8:1.
	{
		Name:   "tokyonight",
		Accent: lipgloss.AdaptiveColor{Dark: "#BB9AF7", Light: "#9854F1"},
		Text:   lipgloss.AdaptiveColor{Dark: "#C0CAF5", Light: "#3760BF"},
		Dim:    lipgloss.AdaptiveColor{Dark: "#737AA2", Light: "#6172B0"},
		Border: lipgloss.AdaptiveColor{Dark: "#3B4261", Light: "#A1A6C5"},
		Green:  lipgloss.AdaptiveColor{Dark: "#9ECE6A", Light: "#587539"},
		Amber:  lipgloss.AdaptiveColor{Dark: "#E0AF68", Light: "#8C6C3E"},
		Red:    lipgloss.AdaptiveColor{Dark: "#F7768E", Light: "#F52A65"},
		SelBg:  lipgloss.AdaptiveColor{Dark: "#283457", Light: "#C4C8DA"},
	},
	// catppuccin: pastel. Dark is Mocha, light is Latte
	// (https://github.com/catppuccin/catppuccin). The light selection is
	// Latte's mantle rather than its crust, which is a heavy band on a white
	// terminal. Latte's yellow is a low-contrast palette value by design, so
	// the duplicate row leans on its ▲ and its "(duplicate)" suffix as much as
	// on the color — changing the hue here would stop this being Latte.
	{
		Name:   "catppuccin",
		Accent: lipgloss.AdaptiveColor{Dark: "#CBA6F7", Light: "#8839EF"},
		Text:   lipgloss.AdaptiveColor{Dark: "#CDD6F4", Light: "#4C4F69"},
		Dim:    lipgloss.AdaptiveColor{Dark: "#A6ADC8", Light: "#6C6F85"},
		Border: lipgloss.AdaptiveColor{Dark: "#45475A", Light: "#CCD0DA"},
		Green:  lipgloss.AdaptiveColor{Dark: "#A6E3A1", Light: "#40A02B"},
		Amber:  lipgloss.AdaptiveColor{Dark: "#F9E2AF", Light: "#DF8E1D"},
		Red:    lipgloss.AdaptiveColor{Dark: "#F38BA8", Light: "#D20F39"},
		SelBg:  lipgloss.AdaptiveColor{Dark: "#313244", Light: "#E6E9EF"},
	},
	// mono: grayscale, no accent hue, so lightness carries everything colour
	// normally would. The ramp runs dim → running → text → duplicate → danger
	// → accent, which puts the duplicate row BRIGHTER than an ordinary row: a
	// duplicate is something to notice, and a grey row next to grey rows is
	// not. Running sits just under the row text so a healthy service reads as
	// unremarkable, and danger stops short of accent so the two stay apart.
	//
	// The whole ramp was re-spread so Red is no longer the same value as
	// Accent. Six values now share one lightness axis, so the gaps are tighter
	// than before: measured with go-colorful against a 0.08 Lab floor, the
	// worst tested pair is 0.100. Every value stays ≥4.5:1 on the terminal
	// background and ≥3.37:1 against the selected row.
	//
	// The light half's Text/Amber gap is only 0.058 — below the floor the
	// semantic triple is held to — which is exactly why DupBold is set here:
	// weight now carries the duplicate distinction that brightness gave up.
	// Lightness was never the only channel anyway: row() draws ● running,
	// ▲ duplicate and ○ restricted, the duplicate's name carries a literal
	// "(duplicate)" suffix, and the duplicate warnings are bold.
	{
		Name:    "mono",
		Accent:  lipgloss.AdaptiveColor{Dark: "#FFFFFF", Light: "#000000"},
		Text:    lipgloss.AdaptiveColor{Dark: "#ABABAB", Light: "#454545"},
		Dim:     lipgloss.AdaptiveColor{Dark: "#757575", Light: "#707070"},
		Border:  lipgloss.AdaptiveColor{Dark: "#4A4A4A", Light: "#BEBEBE"},
		Green:   lipgloss.AdaptiveColor{Dark: "#8F8F8F", Light: "#525252"},
		Amber:   lipgloss.AdaptiveColor{Dark: "#C6C6C6", Light: "#383838"},
		Red:     lipgloss.AdaptiveColor{Dark: "#E2E2E2", Light: "#1C1C1C"},
		SelBg:   lipgloss.AdaptiveColor{Dark: "#242424", Light: "#E8E8E8"},
		DupBold: true,
	},
}

func themeByName(name string) (Theme, bool) {
	for _, t := range themeOrder {
		if t.Name == name {
			return t, true
		}
	}
	return Theme{}, false
}

// nextTheme steps delta places from name, wrapping both ways. An unknown name
// starts the walk from the default.
func nextTheme(name string, delta int) Theme {
	at := 0
	for i, t := range themeOrder {
		if t.Name == name {
			at = i
			break
		}
	}
	n := len(themeOrder)
	return themeOrder[((at+delta)%n+n)%n]
}

// ThemeNames lists the registered themes in cycle order.
func ThemeNames() []string {
	names := make([]string, len(themeOrder))
	for i, t := range themeOrder {
		names[i] = t.Name
	}
	return names
}

// ThemeExists reports whether name is a registered theme.
func ThemeExists(name string) bool {
	_, ok := themeByName(name)
	return ok
}
