package ui

// binding is one list-mode key and its two descriptions: desc is the long one
// the /help panel shows, hint is the two-column pair the footer squeezes in.
// An empty hint means the binding is help-only.
type binding struct {
	keys string
	desc string
	hint [2]string
}

// keymap is the single source for both the footer hint line and /help, so a
// binding can never be documented in one place and forgotten in the other.
//
// The order is the hint order, not a thematic one: keys() walks the table and
// stops once a pair would not fit, so the first entries are the ones that have
// to survive the narrowest terminal. Moving, / and q come first for exactly
// that reason. Help-only bindings follow, in the order updateList handles them.
//
// Measured against the width loop in keys(): 6 pairs fit at w=60, 8 at w=80,
// all 11 at w=120 (102 columns).
var keymap = []binding{
	{keys: "↑ ↓ / k j", desc: "move the cursor", hint: [2]string{"↑↓", "move"}},
	{keys: "/", desc: "open the command input", hint: [2]string{"/", "cmds"}},
	{keys: "q", desc: "quit procdeck", hint: [2]string{"q", "quit"}},
	{keys: "f", desc: "filter the list by name, project, port or command", hint: [2]string{"f", "filter"}},
	{keys: "x", desc: "end the selected service, after a confirm", hint: [2]string{"x", "end"}},
	{keys: "r", desc: "restart the selected service in a new window", hint: [2]string{"r", "restart"}},
	{keys: "a", desc: "show every process, not just the coding ones", hint: [2]string{"a", "all"}},
	{keys: "D", desc: "end every duplicate service at once", hint: [2]string{"D", "dups"}},
	{keys: "o / enter", desc: "open the service's link in a browser", hint: [2]string{"o", "open"}},
	{keys: "c", desc: "copy the service's command line", hint: [2]string{"c", "copy"}},
	{keys: "t", desc: "switch to the next color theme", hint: [2]string{"t", "theme"}},
	{keys: "T", desc: "switch to the previous color theme"},
	{keys: "pgup / pgdown", desc: "move the cursor a page at a time"},
	{keys: "home / g", desc: "jump to the first row"},
	{keys: "end / G", desc: "jump to the last row"},
	{keys: "esc", desc: "clear an active filter"},
	{keys: "ctrl+c", desc: "cancel what is on screen, twice in a row to exit"},
}
