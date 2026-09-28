[CmdletBinding()]
param(
    [ValidatePattern('^[0-9A-Za-z._-]+$')]
    [string]$Version = 'dev',

    [string]$OutputDirectory = 'release',

    [switch]$SkipChecks
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = [System.IO.Path]::GetFullPath($PSScriptRoot)
$outputRoot = if ([System.IO.Path]::IsPathRooted($OutputDirectory)) {
    [System.IO.Path]::GetFullPath($OutputDirectory)
} else {
    [System.IO.Path]::GetFullPath((Join-Path $repoRoot $OutputDirectory))
}
$packageName = "show-me-the-story-$Version-windows-amd64"
$zipPath = Join-Path $outputRoot "$packageName.zip"
$stageRoot = Join-Path $outputRoot ('.package-' + [guid]::NewGuid().ToString('N'))

function Assert-LastExitCode([string]$Step) {
    if ($LASTEXITCODE -ne 0) {
        throw "$Step failed with exit code $LASTEXITCODE."
    }
}

function Find-Wails {
    $command = Get-Command wails -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    $goPath = (& go env GOPATH).Trim()
    Assert-LastExitCode 'Resolving GOPATH'
    $candidate = Join-Path $goPath 'bin\wails.exe'
    if (Test-Path -LiteralPath $candidate -PathType Leaf) {
        return $candidate
    }

    throw 'Wails CLI was not found. Run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0'
}

foreach ($tool in @('go', 'npm')) {
    if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
        throw "$tool was not found in PATH."
    }
}
$wails = Find-Wails

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
New-Item -ItemType Directory -Path $stageRoot | Out-Null

try {
    Write-Host '[package] Installing and building frontend...'
    Push-Location (Join-Path $repoRoot 'frontend')
    try {
        & npm ci
        Assert-LastExitCode 'npm ci'
        & npm run build
        Assert-LastExitCode 'Frontend build'
        if (-not $SkipChecks) {
            & npm run check
            Assert-LastExitCode 'Frontend checks'
        }
    } finally {
        Pop-Location
    }

    if (-not $SkipChecks) {
        Write-Host '[package] Running Go tests and vet...'
        Push-Location $repoRoot
        try {
            & go test ./...
            Assert-LastExitCode 'Go tests'
            & go vet ./...
            Assert-LastExitCode 'Go vet'
        } finally {
            Pop-Location
        }
    }

    Write-Host '[package] Building Windows Wails GUI...'
    Push-Location $repoRoot
    try {
        $ldflags = "-s -w -X main.version=$Version"
        & $wails build -s -platform windows/amd64 -webview2 embed -ldflags $ldflags
        Assert-LastExitCode 'Wails build'
    } finally {
        Pop-Location
    }

    $binary = Join-Path $repoRoot 'build\bin\show-me-the-story.exe'
    if (-not (Test-Path -LiteralPath $binary -PathType Leaf)) {
        throw "Built executable was not found: $binary"
    }

    Write-Host '[package] Preparing portable package...'
    Copy-Item -LiteralPath $binary -Destination (Join-Path $stageRoot 'show-me-the-story.exe')
    foreach ($file in @('README.md', 'README.zh.md', 'LICENSE')) {
        $source = Join-Path $repoRoot $file
        if (Test-Path -LiteralPath $source -PathType Leaf) {
            Copy-Item -LiteralPath $source -Destination (Join-Path $stageRoot $file)
        }
    }
    $docsTarget = Join-Path $stageRoot 'docs'
    New-Item -ItemType Directory -Path $docsTarget | Out-Null
    foreach ($file in @('guide.en.md', 'guide.zh.md')) {
        Copy-Item -LiteralPath (Join-Path $repoRoot "docs\$file") -Destination (Join-Path $docsTarget $file)
    }

    # Local stories, API keys, logs, shared cards, and user Skills are
    # intentionally excluded from distributable packages.
    Compress-Archive -Path (Join-Path $stageRoot '*') -DestinationPath $zipPath -CompressionLevel Optimal -Force
    Write-Host "[package] Created: $zipPath"
} finally {
    $stageFull = [System.IO.Path]::GetFullPath($stageRoot)
    $expectedPrefix = $outputRoot.TrimEnd([System.IO.Path]::DirectorySeparatorChar) + [System.IO.Path]::DirectorySeparatorChar
    if ($stageFull.StartsWith($expectedPrefix, [System.StringComparison]::OrdinalIgnoreCase) -and
        [System.IO.Path]::GetFileName($stageFull).StartsWith('.package-', [System.StringComparison]::Ordinal)) {
        Remove-Item -LiteralPath $stageFull -Recurse -Force -ErrorAction SilentlyContinue
    }
}
