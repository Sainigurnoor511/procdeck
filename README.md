# procdeck

A terminal process manager for dev machines. It finds the coding-related
processes already running, gives each a readable name, and lets you end or
restart them with one key.

```
 procdeck                                              5 services · ▲ 1 duplicate
╭──────────────────────────────────────────────────────────────────────────────╮
│   NAME             PROJECT             PORT   LINK                   CPU  MEM │
│ ● uvicorn          ai-dialer-backend   8000   http://localhost:8000  2.0% 180M │
│ ● celery worker    ai-dialer-backend   —                             0.1%  95M │
│ ▲ celery worker…   ai-dialer-backend   —                             0.0%  90M │
│ ● livekit agent    ai-dialer-backend   8081   http://localhost:8081  1.2% 210M │
│ ● next dev         ai-dialer-frontend  3000   http://localhost:3000  4.0% 400M │
╰──────────────────────────────────────────────────────────────────────────────╯
```

## What it shows

- **Name** worked out from the command line: `celery worker`, `celery beat`,
  `uvicorn`, `django`, `livekit agent`, `next dev`, `vite`, `nest`, `redis`,
  `postgres`, `ngrok`, `docker` and more. Anything else run by a known runtime
  shows as `python: script.py`, `node: server.js` and so on.
- **Project**: the folder the process was started in.
- **Port** and a clickable **localhost link** (Windows Terminal, VS Code terminal).
- CPU, memory, uptime.
- **Duplicates**: two independent copies of the same service in the same
  folder. The copy started from a virtualenv (or the oldest) is kept; the
  others are flagged with `▲`.

A service is a whole process tree shown as one row. A uvicorn reloader and its
worker, an `npm run dev` wrapper and the tool it started, and the Windows
virtualenv `python.exe` launcher and the real interpreter it spawns are each
one service.

Editor internals (VS Code helpers, language servers, MCP servers) are hidden;
press `a` to see every process.

## Keys

| Key | Action |
|---|---|
| `↑` `↓` `j` `k` | move |
| `r` | restart in a new console window, same command, folder and environment |
| `x` | end the whole process tree |
| `D` | end all duplicates |
| `o` / `enter` | open the link in the browser |
| `c` | copy the command line |
| `/` | filter by name, project, port or command |
| `a` | toggle all processes |
| `q` | quit |

End and restart ask for `y` first.

## Install

Needs Go 1.27 or newer.

```
go install ./cmd/procdeck
```

This puts `procdeck.exe` in `%USERPROFILE%\go\bin`. Then run `procdeck`.

`procdeck -list` prints the services once and exits; add `-all` for everything.

## Develop

```
go test ./...
go run ./cmd/procdeck
```

| Package | Job |
|---|---|
| `internal/scan` | snapshot of processes and listening TCP ports |
| `internal/classify` | command line to readable name |
| `internal/group` | process trees to services, project, duplicates |
| `internal/actions` | end, restart, open link, copy |
| `internal/ui` | Bubble Tea screen |

To teach it a new service name, add a rule to `Classify` in
`internal/classify/classify.go` and a case to its test table.

Windows is the supported platform. The code builds elsewhere but is untested there.
