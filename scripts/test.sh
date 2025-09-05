#!/usr/bin/env bash
set -e

echo "Running tests..."
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

if [ "$1" == "coverage" ]; then
    echo "Generating coverage report..."
    go tool cover -html=coverage.txt -o coverage.html
    echo "✅ Coverage report: coverage.html"
fi