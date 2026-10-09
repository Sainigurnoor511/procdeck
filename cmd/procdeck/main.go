package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"

	"procdeck/internal/group"
	"procdeck/internal/scan"
	"procdeck/internal/ui"
)

func main() {
	list := flag.Bool("list", false, "print services once and exit")
	all := flag.Bool("all", false, "with -list, include tooling and unclassified processes")
	theme := flag.String("theme", "", "start with a named color theme for this run only; ignored with -list")
	listThemes := flag.Bool("themes", false, "list the color theme names and exit")
	flag.Parse()

	if *listThemes {
		for _, name := range ui.ThemeNames() {
			fmt.Println(name)
		}
		return
	}

	if *list {
		if err := printList(*all); err != nil {
			fmt.Fprintln(os.Stderr, "procdeck:", err)
			os.Exit(1)
		}
		return
	}

	program := tea.NewProgram(startModel(*theme), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "procdeck:", err)
		os.Exit(1)
	}
}

// startModel builds the TUI model with its remembered settings resolved. An
// explicit -theme is a one-off override for this run and does not overwrite
// the preference t / T saved; an unknown name is rejected rather than quietly
// ignored. The scan interval always comes from the config, so an overridden
// theme does not cost the user their interval.
func startModel(theme string) ui.Model {
	model := ui.New(scan.New())
	if theme != "" && !ui.ThemeExists(theme) {
		fmt.Fprintf(os.Stderr, "procdeck: unknown theme %q\nvalid themes: %s\n",
			theme, strings.Join(ui.ThemeNames(), ", "))
		os.Exit(1)
	}
	name, seconds, warning := ui.LoadConfig()
	if theme != "" {
		name, warning = theme, ""
	}
	model = model.WithTheme(name).WithRefreshSeconds(seconds)
	if warning != "" {
		model = model.WithWarning(warning)
	}
	return model
}

func printList(all bool) error {
	snap, err := scan.New().Snapshot()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PID\tNAME\tPROJECT\tPORTS\tPROCS\tMEM\tDUP\tLINK")
	for _, s := range group.Build(snap, group.Options{All: all}) {
		dup := ""
		if s.Duplicate {
			dup = "yes"
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%v\t%d\t%dM\t%s\t%s\n",
			s.Root.PID, s.Name, s.Project, s.Ports, len(s.Procs), s.Mem>>20, dup, s.Link)
	}
	return w.Flush()
}
