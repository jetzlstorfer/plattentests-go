#!/bin/bash

# Mac OS
env GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=0.0.1 -X main.date=`date -u +%Y%m%d-%H:%M`" -o ./bin/plattentestsgo

# Linux
env GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=0.0.1 -X main.date=`date -u +%Y%m%d-%H:%M`" -o ./bin/plattentestsgo-linux

# Windows  
env GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=0.0.1 -X main.date=`date -u +%Y%m%d-%H:%M`" -o ./bin/plattentestsgo.exe
