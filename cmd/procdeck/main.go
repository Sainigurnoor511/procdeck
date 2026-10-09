package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"

	"procdeck/internal/group"
	"procdeck/internal/scan"
	"procdeck/internal/ui"
)

func main() {
	list := flag.Bool("list", false, "print services once and exit")
	all := flag.Bool("all", false, "with -list, include tooling and unclassified processes")
	flag.Parse()

	if *list {
		if err := printList(*all); err != nil {
			fmt.Fprintln(os.Stderr, "procdeck:", err)
			os.Exit(1)
		}
		return
	}

	program := tea.NewProgram(ui.New(scan.New()), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "procdeck:", err)
		os.Exit(1)
	}
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
