@echo off
REM Batch Script to Run Beds Management Integration Tests
REM Usage: run_beds_tests.bat

echo ========================================
echo Tab 6 Beds Management - Integration Tests
echo ========================================
echo.

REM Check if TEST_DATABASE_DSN is set
if "%TEST_DATABASE_DSN%"=="" (
    echo ERROR: TEST_DATABASE_DSN environment variable is not set!
    echo.
    echo Please set it first:
    echo set TEST_DATABASE_DSN=sqlserver://localhost:1433?database=YOUR_DB^&user id=YOUR_USER^&password=YOUR_PASS
    echo.
    pause
    exit /b 1
)

echo Database DSN: %TEST_DATABASE_DSN%
echo.

echo Choose test to run:
echo [1] Main Integration Test (GetBedsByRoom)
echo [2] Data Integrity Tests
echo [3] Empty Room Tests
echo [4] Upsert Tests
echo [5] ALL Tests
echo [Q] Quit
echo.
set /p choice="Enter choice (1-5 or Q): "

if /i "%choice%"=="Q" exit /b 0

if "%choice%"=="1" (
    echo Running MAIN integration test...
    echo.
    go test -v ./internal/repository -run TestGetBedsByRoom_Integration
    goto :end
)

if "%choice%"=="2" (
    echo Running DATA INTEGRITY tests...
    echo.
    go test -v ./internal/repository -run TestGetBedsByRoom_DataIntegrity
    goto :end
)

if "%choice%"=="3" (
    echo Running EMPTY ROOM tests...
    echo.
    go test -v ./internal/repository -run TestGetBedsByRoom_EmptyClassRoom
    goto :end
)

if "%choice%"=="4" (
    echo Running UPSERT tests...
    echo.
    go test -v ./internal/repository -run TestUpsertBeds
    goto :end
)

if "%choice%"=="5" (
    echo Running ALL beds management tests...
    echo.
    go test -v ./internal/repository -run "TestGetBedsByRoom|TestUpsertBeds"
    goto :end
)

echo Invalid choice!
goto :end

:end
echo.
pause
