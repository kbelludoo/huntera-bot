@echo off
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
echo Compilando huntera-bot para Linux AMD64...
go build -ldflags="-s -w" -o huntera-bot-linux .
echo Concluido!
