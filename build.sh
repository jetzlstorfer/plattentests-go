#!/bin/bash

#VERSION=$(cat version | tr -d '[:space:]')
#go build -ldflags="-X 'main.Version=$VERSION'"  -o keptn

# MAC
env GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=0.0.1 -X main.date=`date -u +%Y%m%d-%H:%M`" -o ./bin/plattentestsgo

# Linux
env GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=0.0.1 -X main.date=`date -u +%Y%m%d-%H:%M`" -o ./bin/plattentestsgo-linux

# Windows build 
env GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=0.0.1 -X main.date=`date -u +%Y%m%d-%H:%M`" -o ./bin/plattentestsgo.exe
