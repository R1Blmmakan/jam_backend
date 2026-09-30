# run.ps1 - Run BitChord Listen Together Server locally
param(
    [int]$Port = 8080
)

$env:PORT = $Port
$env:TRUST_PROXY = "false"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Go is not installed or not in PATH." -ForegroundColor Yellow
    Write-Host "To install Go on Windows, run in PowerShell:" -ForegroundColor Cyan
    Write-Host "  winget install GoLang.Go" -ForegroundColor Green
    Write-Host "Or download from https://go.dev/dl/" -ForegroundColor Cyan
    exit 1
}

Write-Host "Starting BitChord Listen Together Server on port $Port..." -ForegroundColor Green
go run .
