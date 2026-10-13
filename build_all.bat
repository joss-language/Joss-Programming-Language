@echo off
echo Building JosSecurity Distribution...
powershell -ExecutionPolicy Bypass -File build_all.ps1
if %ERRORLEVEL% NEQ 0 (
    echo Build failed!
    exit /b %ERRORLEVEL%
)
echo Build complete.
pause
