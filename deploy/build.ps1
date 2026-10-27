param(
    [ValidateSet('armv7', 'arm64')]
    [string]$Target = 'armv7',
    [string]$Go = 'go'
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $projectRoot 'dist'
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
$savedGoos = $env:GOOS
$savedGoarch = $env:GOARCH
$savedGoarm = $env:GOARM
$savedCgo = $env:CGO_ENABLED
try {
    $env:GOOS = 'linux'
    $env:CGO_ENABLED = '0'
    if ($Target -eq 'armv7') {
        $env:GOARCH = 'arm'
        $env:GOARM = '7'
    } else {
        $env:GOARCH = 'arm64'
        Remove-Item Env:GOARM -ErrorAction SilentlyContinue
    }
    $output = Join-Path $outputDirectory "delegao-linux-$Target"
    Push-Location $projectRoot
    try {
        & $Go build -trimpath '-ldflags=-s -w' -o $output .
        if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }
    } finally { Pop-Location }
    Get-FileHash -LiteralPath $output -Algorithm SHA256
} finally {
    $env:GOOS = $savedGoos
    $env:GOARCH = $savedGoarch
    $env:GOARM = $savedGoarm
    $env:CGO_ENABLED = $savedCgo
}
