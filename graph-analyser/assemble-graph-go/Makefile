# Makefile for graph assembler

.PHONY: build run clean test

# Build the Go binary
build:
	go build -o assemble-graph-optimized assemble-graph-optimized.go

# Run the program
run: build
	./assemble-graph-optimized

# Clean build artifacts
clean:
	rm -f assemble-graph-optimized
	rm -f *.json
	rm -f *.csv

# Test compilation
test:
	go build -o /dev/null assemble-graph-optimized.go

# Run with verbose output
run-verbose: build
	./assemble-graph-optimized 2>&1 | tee graph-assembly.log

# Show help
help:
	@echo "Available targets:"
	@echo "  build       - Compile the Go program"
	@echo "  run         - Build and run the program"
	@echo "  run-verbose - Run with verbose output and logging"
	@echo "  test        - Test compilation"
	@echo "  clean       - Remove build artifacts and output files"
	@echo "  help        - Show this help message"