<#
.SYNOPSIS
    Build, install and run procdeck.

.DESCRIPTION
    By default this compiles the current source to .\bin\procdeck.exe and
    launches it, so what you see is always the working tree rather than a
    stale binary.

.PARAMETER Install
    Also run "go install ./cmd/procdeck", putting procdeck.exe in
    $env:GOBIN (or $env:USERPROFILE\go\bin) so the bare "procdeck"
    command on your PATH is updated too.

.PARAMETER Test
    Run "go test ./..." before building. Build is skipped if tests fail.

.PARAMETER List
    Launch with -list instead of the TUI: prints the services once and exits.

.PARAMETER All
    With -List, include every process, not just the coding-related ones.

.PARAMETER NoRun
    Build (and install, if asked) but don't launch anything.

.EXAMPLE
    .\build.ps1
    Build and open the TUI.

.EXAMPLE
    .\build.ps1 -Test -Install
    Test, build, update the binary on your PATH, then open the TUI.

.EXAMPLE
    .\build.ps1 -List -All
    Build and dump every process to stdout.
#>

[CmdletBinding()]
param(
    [switch]$Install,
    [switch]$Test,
    [switch]$List,
    [switch]$All,
    [switch]$NoRun
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Always operate on the repo this script lives in, whatever the caller's cwd.
$repo = $PSScriptRoot
Push-Location $repo

try {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw 'go was not found on PATH. Install Go 1.27 or newer: https://go.dev/dl/'
    }

    if ($Test) {
        Write-Host '==> go test ./...' -ForegroundColor Cyan
        & go test ./...
        if ($LASTEXITCODE -ne 0) {
            throw "tests failed (exit $LASTEXITCODE); not building"
        }
    }

    $outDir = Join-Path $repo 'bin'
    $exe = Join-Path $outDir 'procdeck.exe'
    if (-not (Test-Path $outDir)) {
        New-Item -ItemType Directory -Path $outDir | Out-Null
    }

    Write-Host "==> go build -o $exe ./cmd/procdeck" -ForegroundColor Cyan
    & go build -o $exe ./cmd/procdeck
    if ($LASTEXITCODE -ne 0) {
        throw "build failed (exit $LASTEXITCODE)"
    }

    $stamp = (Get-Item $exe).LastWriteTime.ToString('HH:mm:ss')
    Write-Host "    built $exe at $stamp" -ForegroundColor DarkGray

    if ($Install) {
        Write-Host '==> go install ./cmd/procdeck' -ForegroundColor Cyan
        & go install ./cmd/procdeck
        if ($LASTEXITCODE -ne 0) {
            throw "install failed (exit $LASTEXITCODE)"
        }

        $binDir = if ($env:GOBIN) { $env:GOBIN } else { Join-Path $env:USERPROFILE 'go\bin' }
        Write-Host "    installed to $binDir" -ForegroundColor DarkGray

        $onPath = ($env:PATH -split ';' | Where-Object { $_ -and ($_.TrimEnd('\') -ieq $binDir.TrimEnd('\')) })
        if (-not $onPath) {
            Write-Warning "$binDir is not on PATH, so the bare 'procdeck' command won't resolve. Add it, or run $exe directly."
        }
    }

    if ($NoRun) {
        return
    }

    $runArgs = @()
    if ($List) {
        $runArgs += '-list'
        if ($All) { $runArgs += '-all' }
    }
    elseif ($All) {
        Write-Warning '-All only applies with -List; ignoring. Press "a" inside the TUI to toggle all processes.'
    }

    if ($runArgs.Count -gt 0) {
        Write-Host "==> $exe $($runArgs -join ' ')" -ForegroundColor Cyan
    }
    else {
        Write-Host "==> $exe" -ForegroundColor Cyan
    }

    & $exe @runArgs
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
