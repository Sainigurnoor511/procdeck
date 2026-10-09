package classify

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		exe, cmd string
		want     string
		ok       bool
	}{
		{"python.exe", `"E:\app\.venv\Scripts\python.exe" -m celery -A app.celery_app worker --pool=threads`, "celery worker", true},
		{"python.exe", `python -m celery -A app.celery_app beat --loglevel=warning`, "celery beat", true},
		{"celery.exe", `celery -A app flower`, "celery flower", true},
		{"python.exe", `python -m uvicorn main:app --host 0.0.0.0 --port 8000`, "uvicorn", true},
		{"uvicorn.exe", `uvicorn main:app --reload`, "uvicorn", true},
		{"python.exe", `python -m gunicorn app:app`, "gunicorn", true},
		{"python.exe", `python manage.py runserver 0.0.0.0:8000`, "django", true},
		{"python.exe", `python -m flask run`, "flask", true},
		{"python.exe", `python -m streamlit run dash.py`, "streamlit", true},
		{"jupyter-lab.exe", `jupyter-lab`, "jupyter", true},
		{"python.exe", `python livekit_agent_worker.py dev`, "livekit agent", true},
		{"python.exe", `python -m pytest tests/`, "pytest", true},
		{"node.exe", `"C:\Program Files\nodejs\node.exe" E:\web\node_modules\next\dist\bin\next dev`, "next dev", true},
		{"node.exe", `node E:\web\node_modules\next\dist\bin\next start`, "next start", true},
		{"node.exe", `node E:\web\node_modules\vite\bin\vite.js`, "vite", true},
		{"node.exe", `node E:\api\node_modules\@nestjs\cli\bin\nest.js start --watch`, "nest", true},
		{"node.exe", `node E:\web\node_modules\@angular\cli\bin\ng.js serve`, "angular", true},
		{"node.exe", `node E:\api\node_modules\tsx\dist\cli.mjs watch src/server.ts`, "tsx: server.ts", true},
		{"node.exe", `node E:\api\node_modules\.bin\tsx watch src/server.ts`, "tsx: server.ts", true},
		{"node.exe", `node "C:\Program Files\nodejs\node_modules\npm\bin\npm-cli.js" run dev`, "npm: dev", true},
		{"node.exe", `node C:\x\pnpm.cjs dev`, "pnpm: dev", true},
		{"bun.exe", `bun run dev`, "bun: dev", true},
		{"python.exe", `python scripts\seed_db.py --force`, "python: seed_db.py", true},
		{"python.exe", `python -m pipeline.shell consolidate`, "python: pipeline.shell", true},
		{"python.exe", `python -c "from multiprocessing.spawn import spawn_main; spawn_main(parent_pid=1)"`, "python", true},
		{"python3.11.exe", `python3.11 worker.py`, "python: worker.py", true},
		{"node.exe", `node server.js`, "node: server.js", true},
		{"redis-server.exe", `redis-server`, "redis", true},
		{"postgres.exe", `postgres -D data`, "postgres", true},
		{"ngrok.exe", `ngrok http 8000`, "ngrok", true},
		{"chrome.exe", `chrome --type=renderer`, "", false},
		{"notepad.exe", `notepad uvicorn.txt`, "", false},
		{"Code.exe", `Code.exe E:\web\vite`, "", false},
	}
	for _, c := range cases {
		got, ok := Classify(c.exe, c.cmd)
		if ok != c.ok || got.Name != c.want {
			t.Errorf("Classify(%q, %q) = %q, %v; want %q, %v", c.exe, c.cmd, got.Name, ok, c.want, c.ok)
		}
	}
}

func TestMatchFlags(t *testing.T) {
	if m, _ := Classify("node.exe", `node npm-cli.js run dev`); !m.Wrapper {
		t.Error("npm runner should be a wrapper")
	}
	if m, _ := Classify("python.exe", `python seed.py`); !m.Generic {
		t.Error("fallback name should be generic")
	}
	if m, _ := Classify("python.exe", `python -m uvicorn main:app`); m.Generic || m.Wrapper {
		t.Error("uvicorn should be neither generic nor wrapper")
	}
}

func TestIsTooling(t *testing.T) {
	tooling := []string{
		`node C:\Users\x\.vscode\extensions\ms-python.vscode-pylance\dist\server.js`,
		`node C:\Users\x\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\cli.js`,
		`node E:\web\node_modules\typescript\lib\tsserver.js`,
		`node C:\x\some-mcp-server\index.js`,
	}
	for _, c := range tooling {
		if !IsTooling("", c) {
			t.Errorf("expected tooling: %s", c)
		}
	}
	if IsTooling(`E:\app\.venv\Scripts\python.exe`, `python -m uvicorn main:app`) {
		t.Error("uvicorn must not be tooling")
	}
}

func TestSplit(t *testing.T) {
	got := Split(`"C:\Program Files\nodejs\node.exe"  a "b c"`)
	want := []string{`C:\Program Files\nodejs\node.exe`, "a", "b c"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d = %q, want %q", i, got[i], want[i])
		}
	}
}
