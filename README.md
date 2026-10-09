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
| `pgup` / `pgdown` | move a page |
| `home` / `g`, `end` / `G` | jump to the first / last row |
| `r` | restart in a new console window, same command, folder and environment |
| `x` | end the whole process tree |
| `D` | end all duplicates |
| `o` / `enter` | open the link in the browser |
| `c` | copy the command line |
| `/` | open the command input (slash commands) |
| `f` | filter by name, project, port or command |
| `esc` | clear an active filter |
| `a` | toggle all processes |
| `t` / `T` | next / previous color theme |
| `q` | quit |
| `ctrl+c` | cancel what is on screen; press again within 1.5 s to exit |

End and restart ask for `y` first.

Press `/help` inside procdeck for the same table plus every command. That panel
is generated from the keymap and the command registry, so it cannot fall behind
this README.

## Slash commands

`/` opens a command input with fuzzy-matched suggestions. `tab` completes the
highlighted one, `enter` runs it, `esc` or backspacing past the `/` closes it.
Once you type a space the suggestions close and `enter` takes the literal text,
so arguments work. A bare `/` followed by `enter` opens `/help`, the first entry
in the registry.

| Command | Does |
|---|---|
| `/help` | list every key and every command, scrollable |
| `/theme [name]` | switch the color theme, or cycle with no name |
| `/themes` | list the available color themes |
| `/filter <text>` | narrow the list to matching services |
| `/all` | toggle showing every process, not just coding ones |
| `/clear` | clear the filter and the status line |
| `/refresh [seconds]` | show or set how often procdeck rescans |
| `/end [name]` | end a service, after a confirm |
| `/restart [name]` | restart a service in a new window |
| `/end-duplicates` | end every duplicate service at once |
| `/open [name]` | open a service's link in a browser |
| `/copy [name]` | copy a service's command line |
| `/config` | show the config file path and the current settings |
| `/doctor` | report the terminal, config and scan diagnostics |
| `/version` | show the procdeck and Go versions |
| `/quit` | quit procdeck |

An optional `[name]` defaults to the row under the cursor and is otherwise a
case-insensitive substring of a visible service's name or project. A name that
matches two services reports both and acts on neither — a slash command never
ends something you did not name, and `/end`, `/restart` and `/end-duplicates`
go through the same `y`/`n` confirm as the `x`, `r` and `D` keys.

An unknown command or a rejected argument always says what was wrong; nothing
fails silently.

`/help` and `/doctor` open a full-height panel in place of the table. `↑` `↓`
`j` `k`, `pgup`/`pgdown` and `home`/`end` scroll it; `esc` or `q` returns to the
list. `/doctor` reports the color profile, the detected background, the terminal
size as procdeck sees it, the config path with its read and write status, the
effective scan interval, the counts from the last scan and how many processes
refused access. Its report is taken when you run the command, not per frame.
The hyperlink row is an honest guess from `WT_SESSION`, `TERM_PROGRAM` and
`VTE_VERSION`: nothing is probed, and procdeck emits the OSC 8 escape either
way.

## Themes

Six palettes ship with procdeck:

| Name | Look |
|---|---|
| `claude` | warm terracotta (the default) |
| `nord` | cool blue-grey |
| `gruvbox` | warm retro |
| `tokyonight` | purple-blue |
| `catppuccin` | pastel |
| `mono` | grayscale, no accent hue |

Press `t` and `T` to walk through them; the name shows in the status line and
the choice is remembered for next time. `procdeck -themes` prints the names,
and `procdeck -theme nord` starts in a named theme for that run only, without
overwriting what `t` saved. An unknown name is rejected rather than ignored.
`-theme` has no effect alongside `-list`.

Every theme follows the terminal's light or dark background, and keeps the
running, duplicate and restricted states distinguishable. `mono` has no hue to
spend, so it leans on the `●` `▲` `○` glyphs and the `(duplicate)` suffix just
as much as on brightness, and it also renders duplicate rows **bold** — the one
theme that does.

## Settings

Remembered settings live in `%APPDATA%\procdeck\config.json`:

```json
{"theme":"nord","refreshIntervalSeconds":2}
```

Those two keys are the whole schema. `refreshIntervalSeconds` is how often
procdeck rescans — `/refresh <seconds>` sets it, the floor is 1 second and the
ceiling is 3600, and a new interval takes effect from the next scan. The key is
omitted while the interval is the default of 2 seconds, so a theme-only
preference still writes just `{"theme":"nord"}`.

Nothing about this file is fatal. A missing one is the normal first run. An
unreadable or malformed file, an unknown theme name, an out-of-range interval
and an unrecognised key each fall back to the default for that one setting and
say so in the status line rather than stopping procdeck from starting. Writes go
to a temp file and are renamed into place, and the two keys are written
independently, so switching a theme cannot forget a configured interval.

`/config` prints the path and the live settings on the status line; `/doctor`
adds whether the file can be read and the directory written.

Palettes come from [Nord](https://www.nordtheme.com/docs/colors-and-palettes/),
[gruvbox](https://github.com/morhetz/gruvbox),
[tokyonight.nvim](https://github.com/folke/tokyonight.nvim) and
[Catppuccin](https://github.com/catppuccin/catppuccin). Nord ships dark-only, so
its light half is derived rather than upstream; a few other values are nudged
for legibility in a box-heavy TUI, and `internal/ui/theme.go` says which and why.

## Install

Needs Go 1.27 or newer.

```
go install ./cmd/procdeck
```

This puts `procdeck.exe` in `$env:GOBIN`, or `%USERPROFILE%\go\bin` when GOBIN
is unset. Then run `procdeck` from anywhere — as long as that directory is on
your `PATH`. If it is not, add it:

```powershell
[Environment]::SetEnvironmentVariable(
    'Path',
    [Environment]::GetEnvironmentVariable('Path', 'User') + ";$env:USERPROFILE\go\bin",
    'User')
```

From a clone, `.\build.ps1 -Install` does the same thing from the working tree:
it builds `.\bin\procdeck.exe`, runs `go install`, prints where the binary
landed, and warns if that directory is not on `PATH`. Add `-Test` to gate the
build on `go test ./...`, and `-NoRun` to stop it launching the TUI afterwards.

`procdeck -list` prints the services once and exits; add `-all` for everything.
`procdeck -themes` lists the color themes.

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
