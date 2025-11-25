@echo off
title Starting IM Project Services

echo Starting API service...
start "API Service" cmd /k "cd api && go run main.go"

echo Starting User service...
start "User Service" cmd /k "cd user && go run main.go"

echo Starting Message service...
start "Message Service" cmd /k "cd message && go run main.go"

echo.
echo All services are starting in separate windows.
pause
