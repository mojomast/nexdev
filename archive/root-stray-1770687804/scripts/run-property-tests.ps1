# Script to run property-based tests for Geoffrussy
# This script runs the state persistence round-trip property tests

$ErrorActionPreference = "Stop"

Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "Running Property-Based Tests" -ForegroundColor Cyan
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

# Check if Go is installed
try {
    $goVersion = go version
    Write-Host "Go version: $goVersion"
    Write-Host ""
} catch {
    Write-Host "Error: Go is not installed" -ForegroundColor Red
    Write-Host "Please install Go from https://golang.org/dl/"
    exit 1
}

# Install dependencies
Write-Host "Installing dependencies..." -ForegroundColor Yellow
go mod download
Write-Host ""

# Run property tests
Write-Host "Running Property 4: State Preservation Round-Trip..." -ForegroundColor Yellow
Write-Host "Minimum iterations: 100"
Write-Host ""

go test -v ./internal/state/... -run TestProperty4_StatePreservationRoundTrip

# Check exit code
if ($LASTEXITCODE -eq 0) {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Green
    Write-Host "✓ All property tests passed!" -ForegroundColor Green
    Write-Host "==========================================" -ForegroundColor Green
} else {
    Write-Host ""
    Write-Host "==========================================" -ForegroundColor Red
    Write-Host "✗ Property tests failed" -ForegroundColor Red
    Write-Host "==========================================" -ForegroundColor Red
    exit 1
}
