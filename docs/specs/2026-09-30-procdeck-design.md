# procdeck — design spec

Date: 2026-09-30

## Purpose

A terminal UI that shows every coding-related process running on a Windows dev
machine under a readable name ("celery worker", "uvicorn", "next dev"), with its
project, port, and localhost link, and lets the user end or restart it with one key.

It exists because a dev stack started twice (once from a project `.venv`, once
from global Python) produced ten anonymous `python.exe` rows in Task Manager,
and nothing made the duplication visible.

Success: open `procdeck`, see at a glance what is running, where, on which
port, and which entries are duplicates; end or restart any of them without
leaving the terminal.

## Scope

In scope:

- Discover processes that are already running. No config file.
- Friendly naming, project detection, port and link display, CPU and memory.
- Duplicate detection.
- Actions: end, end all duplicates, restart, open link, copy command.
- Windows 10/11.

Out of scope:

- Launching a stack from a config file.
- Log tailing.
- Remote hosts.
- Linux and macOS support (code must not prevent it later, but it is not tested).

## Stack

- Go (latest stable), built to a single `procdeck.exe`.
- Bubble Tea (event loop), Lip Gloss (styling), Bubbles (table, text input).
- gopsutil v4 for process and socket data.
- `atotto/clipboard` for copy.

Go is chosen over Rust because it needs no C/MSVC build toolchain on Windows,
compiles in seconds, and gopsutil already covers command line, working
directory, environment and listening sockets on Windows.

## Layout

```
procdeck/
  cmd/procdeck/main.go     entry point
  internal/scan/           snapshot of raw processes and listening ports
  internal/classify/       command line -> friendly name and kind
  internal/group/          process trees -> services, project, duplicates
  internal/actions/        end, restart, open link, copy
  internal/ui/             Bubble Tea model, view, key handling
  go.mod
  README.md
```

Each package depends only on the ones above it in this list; `ui` is the only
package that knows about Bubble Tea.

## Units

### scan

`Snapshot() (Snapshot, error)` returns, for every process the user can read:
pid, parent pid, executable name, executable path, raw command line, working
directory, create time, CPU percent, resident memory, and the list of TCP ports
it is listening on. Processes whose details are access-denied are returned with
`Restricted: true` and whatever fields were readable.

CPU percent is computed from the difference between two consecutive snapshots,
so the scanner keeps the previous CPU times per pid.

### classify

`Classify(exeName, cmdline string) (name string, kind Kind, ok bool)`.
A pure function driven by an ordered rule table. `ok` is false when the process
is not coding-related.

Rules, first match wins:

| Match in command line | Name |
|---|---|
| `celery` + `worker` | celery worker |
| `celery` + `beat` | celery beat |
| `celery` + `flower` | celery flower |
| `uvicorn` | uvicorn |
| `gunicorn` | gunicorn |
| `hypercorn` | hypercorn |
| `manage.py runserver` | django |
| `flask run` | flask |
| `streamlit run` | streamlit |
| `jupyter` | jupyter |
| `livekit` in script name | livekit agent |
| `pytest` | pytest |
| `next dev` / `next start` | next dev / next start |
| `vite` | vite |
| `nest start` | nest |
| `ng serve` | angular |
| `webpack` | webpack |
| `tsx` / `ts-node` / `nodemon` | tsx / ts-node / nodemon (+ script name) |
| `npm run X` / `pnpm X` / `yarn X` / `bun run X` | npm: X (etc.) |
| exe `redis-server`, `postgres`, `mysqld`, `mongod` | redis, postgres, mysql, mongodb |
| exe `ngrok`, `cloudflared`, `livekit-server` | same |
| exe `docker`, `com.docker.backend` | docker |

Fallback for a known runtime (`python`, `node`, `bun`, `deno`, `go`, `java`,
`dotnet`, `ruby`, `php`, `cargo`) with no rule match: runtime name plus the
script file name, e.g. `python: seed_db.py`.

Hidden by default even though they use a known runtime: editor and tooling
internals. A process is tooling when its command line or executable path
contains any of `\.vscode\`, `\Microsoft VS Code\`, `\.cursor\`, `\.claude\`,
`claude-code`, `language-server`, `languageserver`, `tsserver`, `pylance`,
`pyright`, `eslint`, `typescript-language`, `copilot`, `\JetBrains\`, `mcp`.

### group

`Build(snapshot) []Service`.

- A service is a process tree: a classified root and all its descendants. A
  classified process whose parent is classified with the same name is folded
  into the parent (uvicorn reloader and its worker, livekit agent and its
  spawned job processes, multiprocessing children).
- A process that is not classified but is listening on a TCP port, is owned by
  the current user, and is not tooling, becomes a service named after its
  executable.
- Service ports are the union of ports across the tree. CPU and memory are sums.
- Project is the base name of the root's working directory. If the working
  directory is unreadable, project is `?`.
- Link is `http://localhost:<port>` for the lowest port, present only when the
  service has at least one port.
- Variant is `venv` when the executable path contains `\.venv\`, `\venv\` or
  `\env\`, otherwise `global`; only set for Python.
- Duplicates: services sharing the same name and the same project form a
  duplicate set when there are two or more. Within a set the oldest service
  that is a `venv` variant is the keeper; if none is `venv`, the oldest is the
  keeper. All others are marked `Duplicate: true`.
- Sort order: project, then name, then start time.

### actions

- `End(service)`: terminates every process in the tree, children first. Returns
  the list of pids that could not be ended.
- `EndDuplicates(services)`: `End` on every service marked duplicate.
- `Restart(service)`: reads the root's raw command line, working directory and
  environment, ends the tree, waits until every pid has exited (timeout 5 s),
  then starts the same command line in the same directory with the same
  environment in a **new console window** so its output stays visible. If the
  environment cannot be read, the current environment is used and the status
  bar says so. If the command line or working directory cannot be read,
  restart is refused before anything is killed.
- `Open(link)`: opens the link in the default browser.
- `Copy(text)`: puts the root command line on the clipboard.

No action ever targets procdeck's own pid or any of its ancestors. Actions on
a `Restricted` service are refused.

### ui

Single screen.

- Header: app name, service count, duplicate count (warning colour when > 0).
- Table columns: status dot, NAME, PROJECT, PORT, LINK, CPU, MEM, UPTIME.
  Duplicate rows use the warning colour and a `⚠` dot. Restricted rows are dim.
  Links are emitted as OSC 8 hyperlinks so they are clickable in Windows
  Terminal and the VS Code terminal.
- Detail pane for the selected row: pid, variant, child count, command line,
  working directory, all ports.
- Footer: key hints, and a status line for the result of the last action.

Keys:

| Key | Action |
|---|---|
| `↑` `↓` `j` `k` | move selection |
| `r` | restart (asks `y/n`) |
| `x` | end (asks `y/n`) |
| `D` | end all duplicates (asks `y/n`) |
| `o` | open link |
| `c` | copy command line |
| `/` | filter by text across name, project, port |
| `a` | toggle showing hidden tooling and unclassified processes |
| `q` `ctrl+c` | quit |

The list refreshes every 2 seconds in the background. Selection follows the
selected service's root pid across refreshes; if that pid is gone, selection
stays at the same row index.

The layout adapts to terminal width: the LINK column is dropped first, then
UPTIME, when the terminal is too narrow.

## Error handling

- A failed snapshot keeps the previous list on screen and shows the error in
  the status line.
- Access-denied on a single process never fails the snapshot.
- Every action reports success or the specific failure in the status line.
- Restart that kills but fails to relaunch reports the command line so the user
  can start it by hand; the command is also placed on the clipboard.

## Testing

- `classify`: table-driven tests for every rule, the fallback, and tooling
  exclusion.
- `group`: tests over hand-built snapshots, including the real case that
  motivated the tool (uvicorn, celery worker, celery beat and livekit agent,
  each running once from `.venv` and once from global Python, plus a
  multiprocessing child) asserting four services kept, four marked
  duplicate, and the child folded into its parent.
- `actions`: tests that spawn a throwaway long-sleeping process tree, then end
  and restart it.
- `ui`: model-level tests for selection stability, filter, and confirm flow by
  feeding key messages to `Update`.

## Amendments made while building

These replace the matching statements above.

- `Classify` returns a `Match` (name, `Generic`, `Wrapper`) instead of a name and kind.
- A child folds into its nearest classified ancestor when it has the same name,
  when the ancestor is a package-manager wrapper (`npm: dev`), or when the
  child only has a fallback name. A wrapper service takes the name of the real
  tool it started.
- The Windows virtualenv `python.exe` is a launcher that starts the base
  interpreter as a child with the same command line. That pair is one service,
  not a duplicate. The motivating case is therefore four services and no
  duplicates; duplicates are independent trees with the same name and folder.
- Unclassified listening processes are shown only when their executable lives
  outside `\Windows\`, `\Program Files`, `\ProgramData\` and `\AppData\`.
- Restricted processes are still named from their executable, so a Postgres
  Windows service shows as `postgres`, dimmed, with actions disabled.
- `End` kills the root first so a supervisor cannot respawn children, and skips
  any pid whose create time no longer matches.
- `Restart` starts the process directly with `CreateProcess` in a new console.
- `enter` also opens the link. `-list` and `-all` flags print once and exit.
