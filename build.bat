@echo off
setlocal

echo.
echo ==============================
echo Building GoPlay for Windows...
echo ==============================
echo.

where go >nul 2>nul
if errorlevel 1 (
    echo ERROR: Go is not installed or not found in PATH.
    echo Install Go from https://go.dev/dl/ and restart the terminal.
    pause
    exit /b 1
)

echo Checking dependencies...
go mod tidy
if errorlevel 1 (
    echo.
    echo ERROR: go mod tidy failed.
    pause
    exit /b 1
)

echo.
echo Building goplay.exe with console...
go build -o goplay.exe .
if errorlevel 1 (
    echo.
    echo ERROR: build failed.
    pause
    exit /b 1
)

echo.
echo ==============================
echo Build complete!
echo ==============================
echo.
echo Created file:
echo goplay.exe
echo.
echo Run it from terminal or double-click it.
echo To stop GoPlay, close the console window or press Ctrl+C.
echo.
echo Important:
echo Put mpv.exe next to goplay.exe before running GoPlay.
echo.
pause

endlocal