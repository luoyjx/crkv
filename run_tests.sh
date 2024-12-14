#!/bin/bash

# Run tests with coverage for each package
echo "Running tests with coverage..."
go test -v -coverprofile=coverage.out ./...

# Display coverage report
echo -e "\nCoverage Report:"
go tool cover -func=coverage.out

# Generate HTML coverage report
go tool cover -html=coverage.out -o coverage.html
echo -e "\nDetailed coverage report generated in coverage.html"
