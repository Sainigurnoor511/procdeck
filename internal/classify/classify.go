// Package classify turns a process command line into a readable service name.
package classify

import (
	"strings"
)

type Match struct {
	Name string
	// Generic names come from the runtime fallback ("python: seed.py").
	Generic bool
	// Wrapper marks package-manager runners whose real work happens in a child.
	Wrapper bool
}

var infra = map[string]string{
	"redis-server":       "redis",
	"postgres":           "postgres",
	"mysqld":             "mysql",
	"mongod":             "mongodb",
	"ngrok":              "ngrok",
	"cloudflared":        "cloudflared",
	"livekit-server":     "livekit-server",
	"docker":             "docker",
	"com.docker.backend": "docker",
}

var runtimes = map[string]string{
	"python": "python", "python3": "python", "pythonw": "python", "py": "python",
	"node": "node", "bun": "bun", "deno": "deno", "go": "go",
	"java": "java", "javaw": "java", "dotnet": "dotnet",
	"ruby": "ruby", "php": "php", "cargo": "cargo",
}

var tools = map[string]bool{
	"celery": true, "uvicorn": true, "gunicorn": true, "hypercorn": true,
	"flask": true, "streamlit": true, "pytest": true, "next": true,
	"vite": true, "nest": true, "ng": true, "webpack": true, "tsx": true,
	"ts-node": true, "nodemon": true, "npm": true, "npx": true,
	"pnpm": true, "yarn": true,
}

var toolingMarkers = []string{
	`\.vscode\`, `\microsoft vs code`, `\.cursor\`, `\.claude\`, `\claude.exe`,
	"claude-code", "language-server", "languageserver", "tsserver", "pylance",
	"pyright", "eslint", "typescript-language", "copilot", `\jetbrains\`, "mcp",
}

var strippedExts = []string{".exe", ".cmd", ".bat", ".mjs", ".cjs", ".js", ".ps1"}

func IsTooling(exePath, cmdline string) bool {
	hay := strings.ToLower(strings.ReplaceAll(exePath+" "+cmdline, "/", `\`))
	for _, m := range toolingMarkers {
		if strings.Contains(hay, m) {
			return true
		}
	}
	return false
}

// Classify names a process. ok is false when it is not coding-related.
func Classify(exe, cmdline string) (Match, bool) {
	exe = strings.TrimSuffix(strings.ToLower(exe), ".exe")
	if name, ok := infra[exe]; ok {
		return Match{Name: name}, true
	}

	runtime, isRuntime := runtimeOf(exe)
	if !isRuntime && !tools[exe] && !strings.HasPrefix(exe, "jupyter") {
		return Match{}, false
	}

	tokens := Split(cmdline)
	if len(tokens) == 0 {
		tokens = []string{exe}
	}
	words := make([]string, len(tokens))
	pkgs := make([]string, len(tokens))
	for i, t := range tokens {
		words[i] = word(t)
		pkgs[i] = nodePackage(t)
	}
	find := func(w string) int {
		if i := indexOf(words, w); i >= 0 {
			return i
		}
		return indexOf(pkgs, w)
	}
	has := func(w string) bool { return find(w) >= 0 }

	switch {
	case has("celery") && has("worker"):
		return Match{Name: "celery worker"}, true
	case has("celery") && has("beat"):
		return Match{Name: "celery beat"}, true
	case has("celery") && has("flower"):
		return Match{Name: "celery flower"}, true
	case has("uvicorn"):
		return Match{Name: "uvicorn"}, true
	case has("gunicorn"):
		return Match{Name: "gunicorn"}, true
	case has("hypercorn"):
		return Match{Name: "hypercorn"}, true
	case has("manage.py") && has("runserver"):
		return Match{Name: "django"}, true
	case has("flask") && has("run"):
		return Match{Name: "flask"}, true
	case has("streamlit") && has("run"):
		return Match{Name: "streamlit"}, true
	case hasPrefix(words, "jupyter"):
		return Match{Name: "jupyter"}, true
	case runtime == "python" && containsAny(words[1:], "livekit"):
		return Match{Name: "livekit agent"}, true
	case has("pytest"):
		return Match{Name: "pytest"}, true
	case has("next") && has("dev"):
		return Match{Name: "next dev"}, true
	case has("next") && has("start"):
		return Match{Name: "next start"}, true
	case has("vite"):
		return Match{Name: "vite"}, true
	case has("nest") && has("start"):
		return Match{Name: "nest"}, true
	case has("ng") && has("serve"):
		return Match{Name: "angular"}, true
	case has("webpack"):
		return Match{Name: "webpack"}, true
	}

	for _, tool := range []string{"tsx", "ts-node", "nodemon"} {
		if i := find(tool); i >= 0 {
			return Match{Name: join(tool, argAfter(tokens, i, "watch"))}, true
		}
	}
	for _, mgr := range []struct{ word, label string }{
		{"npm-cli", "npm"}, {"npm", "npm"}, {"npx-cli", "npx"}, {"npx", "npx"},
		{"pnpm", "pnpm"}, {"yarn", "yarn"},
	} {
		if i := find(mgr.word); i >= 0 {
			return Match{Name: join(mgr.label, argAfter(tokens, i, "run", "run-script")), Wrapper: true}, true
		}
	}
	if runtime == "bun" && len(words) > 1 && words[1] == "run" {
		return Match{Name: join("bun", argAfter(tokens, 1)), Wrapper: true}, true
	}

	if !isRuntime {
		return Match{Name: exe}, true
	}
	return Match{Name: join(runtime, fallbackLabel(runtime, tokens)), Generic: true}, true
}

func runtimeOf(exe string) (string, bool) {
	if r, ok := runtimes[exe]; ok {
		return r, true
	}
	// versioned interpreters such as python3.11
	if strings.HasPrefix(exe, "python") {
		return "python", true
	}
	return "", false
}

func fallbackLabel(runtime string, tokens []string) string {
	for i := 1; i < len(tokens); i++ {
		t := tokens[i]
		if runtime == "python" {
			if t == "-c" {
				return ""
			}
			if t == "-m" && i+1 < len(tokens) {
				return tokens[i+1]
			}
		}
		if strings.HasPrefix(t, "-") {
			continue
		}
		return base(t)
	}
	return ""
}

func argAfter(tokens []string, i int, skip ...string) string {
	for _, t := range tokens[i+1:] {
		if strings.HasPrefix(t, "-") || indexOf(skip, strings.ToLower(t)) >= 0 {
			continue
		}
		return base(t)
	}
	return ""
}

func join(prefix, label string) string {
	if label == "" {
		return prefix
	}
	return prefix + ": " + label
}

// Split breaks a command line into tokens, honouring double quotes.
func Split(cmdline string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for _, r := range cmdline {
		switch {
		case r == '"':
			inQuote = !inQuote
		case (r == ' ' || r == '\t') && !inQuote:
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

func base(token string) string {
	token = strings.ReplaceAll(token, "/", `\`)
	if i := strings.LastIndex(token, `\`); i >= 0 && i < len(token)-1 {
		return token[i+1:]
	}
	return token
}

func nodePackage(token string) string {
	const marker = `node_modules\`
	t := strings.ToLower(strings.ReplaceAll(token, "/", `\`))
	i := strings.LastIndex(t, marker)
	if i < 0 {
		return ""
	}
	parts := strings.Split(t[i+len(marker):], `\`)
	if len(parts) < 2 || parts[0] == ".bin" || strings.HasPrefix(parts[0], "@") {
		return ""
	}
	return parts[0]
}

func word(token string) string {
	w := strings.ToLower(base(token))
	for _, ext := range strippedExts {
		if strings.HasSuffix(w, ext) {
			return strings.TrimSuffix(w, ext)
		}
	}
	return w
}

func indexOf(words []string, w string) int {
	for i, x := range words {
		if x == w {
			return i
		}
	}
	return -1
}

func hasPrefix(words []string, prefix string) bool {
	for _, x := range words {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}

func containsAny(words []string, sub string) bool {
	for _, x := range words {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
