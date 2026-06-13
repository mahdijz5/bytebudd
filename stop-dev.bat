@echo off
echo ============================================
echo   Stopping ByteBudd Development Infrastructure
echo ============================================
echo.

REM Stop infrastructure containers
echo Stopping infrastructure services...
docker-compose -f docker-compose-dev.yml down

echo.
echo ============================================
echo   Infrastructure Stopped
echo ============================================
echo.
echo   Remember to stop your local services:
echo.
echo   - Backend: Press Ctrl+C in the terminal
echo   - Frontend: Press Ctrl+C in the terminal
echo.
pause