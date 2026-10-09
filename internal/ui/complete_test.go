package ui

import (
	"strings"
	"testing"
)

func matchNames(input string) []string {
	hits := matchCommands(input)
	names := make([]string, len(hits))
	for i, c := range hits {
		names[i] = c.name
	}
	return names
}

func TestFuzzyPrefixBeatsMidString(t *testing.T) {
	// "e" is a prefix of end and end-duplicates and a mid-string hit in the
	// rest, so both prefixes must come first however short the other name is
	got := matchNames("e")
	if len(got) < 3 {
		t.Fatalf("e matched %v", got)
	}
	if got[0] != "end" || got[1] != "end-duplicates" {
		t.Errorf("e ranked %v, want end and end-duplicates first", got[:2])
	}
	for _, name := range got[2:] {
		if score, _ := fuzzyScore("e", name); score < 100 {
			t.Errorf("%q scored %d as a mid-string match, want >= 100", name, score)
		}
	}
}

func TestFuzzyShorterNameWinsOnATie(t *testing.T) {
	if got := matchNames("th"); len(got) < 2 || got[0] != "theme" || got[1] != "themes" {
		t.Errorf("th ranked %v, want theme before themes", got)
	}
}

func TestFuzzyMatchesASubsequence(t *testing.T) {
	if got := matchNames("thm"); len(got) == 0 || got[0] != "theme" {
		t.Errorf("thm ranked %v, want theme first", got)
	}
	if _, ok := fuzzyScore("thm", "theme"); !ok {
		t.Error("thm does not match theme as a subsequence")
	}
}

func TestFuzzyIgnoresCase(t *testing.T) {
	lower, upper := matchNames("ref"), matchNames("REF")
	if strings.Join(lower, ",") != strings.Join(upper, ",") {
		t.Errorf("ref ranked %v but REF ranked %v", lower, upper)
	}
	if len(lower) == 0 || lower[0] != "refresh" {
		t.Errorf("ref ranked %v, want refresh first", lower)
	}
	if _, ok := fuzzyScore("ThEmE", "theme"); !ok {
		t.Error("a mixed-case pattern did not match")
	}
}

func TestFuzzyFindsDuplicatesFromDup(t *testing.T) {
	got := matchNames("dup")
	if len(got) != 1 || got[0] != "end-duplicates" {
		t.Errorf("dup ranked %v, want only end-duplicates", got)
	}
}

// TestFuzzyTieBreaksOnRegistryOrder pins the one case where the score cannot
// decide: refresh and restart are both prefix matches of the same length.
func TestFuzzyTieBreaksOnRegistryOrder(t *testing.T) {
	refresh, _ := fuzzyScore("r", "refresh")
	restart, _ := fuzzyScore("r", "restart")
	if refresh != restart {
		t.Fatalf("refresh scored %d and restart %d, so this is no longer a tie", refresh, restart)
	}
	got := matchNames("r")
	if len(got) < 2 || got[0] != "refresh" || got[1] != "restart" {
		t.Errorf("r ranked %v, want refresh then restart", got)
	}
}

func TestFuzzyEmptyInputReturnsTheRegistry(t *testing.T) {
	got := matchNames("")
	if len(got) != len(commandOrder) {
		t.Fatalf("empty input matched %d commands, want %d", len(got), len(commandOrder))
	}
	for i, c := range commandOrder {
		if got[i] != c.name {
			t.Errorf("entry %d is %q, want %q — empty input must keep registry order", i, got[i], c.name)
		}
	}
}

func TestFuzzyNoMatchReturnsNothing(t *testing.T) {
	if got := matchCommands("zzz"); got != nil {
		t.Errorf("zzz matched %v", matchNames("zzz"))
	}
	if _, ok := fuzzyScore("zzz", "theme"); ok {
		t.Error("zzz claims to match theme")
	}
}

func TestFuzzyOrderingIsStableAcrossCalls(t *testing.T) {
	for _, input := range []string{"", "e", "r", "th", "en"} {
		want := strings.Join(matchNames(input), ",")
		for i := 0; i < 3; i++ {
			if got := strings.Join(matchNames(input), ","); got != want {
				t.Fatalf("%q ranked %q then %q", input, want, got)
			}
		}
	}
}

func TestSuggestWindowKeepsTheSelectionVisible(t *testing.T) {
	for _, c := range []struct {
		count, sel, rows, want int
	}{
		{14, 0, 6, 0},
		{14, 3, 6, 0},
		{14, 13, 6, 8},
		{14, 7, 6, 4},
		{3, 2, 6, 0},  // everything fits, so never scroll
		{14, 4, 0, 0}, // no room at all
	} {
		got := suggestWindow(c.count, c.sel, c.rows)
		if got != c.want {
			t.Errorf("suggestWindow(%d, %d, %d) = %d, want %d", c.count, c.sel, c.rows, got, c.want)
		}
		if c.rows > 0 && c.count > c.rows && (c.sel < got || c.sel >= got+c.rows) {
			t.Errorf("suggestWindow(%d, %d, %d) = %d hides the selection", c.count, c.sel, c.rows, got)
		}
	}
}
