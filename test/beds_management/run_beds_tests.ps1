# PowerShell Script to Run Beds Management Integration Tests
# Usage: .\run_beds_tests.ps1

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Tab 6 Beds Management - Integration Tests" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check if TEST_DATABASE_DSN is set
if (-not $env:TEST_DATABASE_DSN) {
    Write-Host "ERROR: TEST_DATABASE_DSN environment variable is not set!" -ForegroundColor Red
    Write-Host ""
    Write-Host "Please set it using:" -ForegroundColor Yellow
    Write-Host "`$env:TEST_DATABASE_DSN=`"sqlserver://localhost:1433?database=YOUR_DB&user id=YOUR_USER&password=YOUR_PASS`"" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Or run with parameter:" -ForegroundColor Yellow
    Write-Host ".\run_beds_tests.ps1 -DSN `"sqlserver://localhost:1433?database=YOUR_DB&user id=YOUR_USER&password=YOUR_PASS`"" -ForegroundColor Yellow
    exit 1
}

Write-Host "Database DSN: $env:TEST_DATABASE_DSN" -ForegroundColor Green
Write-Host ""

# Parse command line arguments
param(
    [string]$TestName = "",
    [switch]$AllTests,
    [switch]$DataIntegrity,
    [switch]$EmptyRoom,
    [switch]$UpsertTest
)

# Determine which tests to run
if ($AllTests) {
    Write-Host "Running ALL beds management tests..." -ForegroundColor Cyan
    Write-Host ""
    go test -v ./internal/repository -run "TestGetBedsByRoom|TestUpsertBeds"
    exit $LASTEXITCODE
}

if ($DataIntegrity) {
    Write-Host "Running DATA INTEGRITY tests..." -ForegroundColor Cyan
    Write-Host ""
    go test -v ./internal/repository -run "TestGetBedsByRoom_DataIntegrity"
    exit $LASTEXITCODE
}

if ($EmptyRoom) {
    Write-Host "Running EMPTY ROOM tests..." -ForegroundColor Cyan
    Write-Host ""
    go test -v ./internal/repository -run "TestGetBedsByRoom_EmptyClassRoom"
    exit $LASTEXITCODE
}

if ($UpsertTest) {
    Write-Host "Running UPSERT tests..." -ForegroundColor Cyan
    Write-Host ""
    go test -v ./internal/repository -run "TestUpsertBeds"
    exit $LASTEXITCODE
}

if ($TestName -ne "") {
    Write-Host "Running specific test: $TestName" -ForegroundColor Cyan
    Write-Host ""
    go test -v ./internal/repository -run $TestName
    exit $LASTEXITCODE
}

# Default: Run main integration test
Write-Host "Running MAIN integration test (GetBedsByRoom)..." -ForegroundColor Cyan
Write-Host ""
go test -v ./internal/repository -run "TestGetBedsByRoom_Integration"

exit $LASTEXITCODE
