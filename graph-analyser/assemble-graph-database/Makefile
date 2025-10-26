.PHONY: build run clean deps help

# Install dependencies
deps:
	go mod tidy

# Build the Go binary
build: deps
	go build -o assemble-graph-database main.go

# Run the program
run: build
	./assemble-graph-database

# Clean build artifacts
clean:
	rm -f assemble-graph-database
	rm -f analysis_queries.sql

help:
	@echo "Available targets:"
	@echo "  deps         - Install Go dependencies" 
	@echo "  build        - Compile the Go program"
	@echo "  run          - Build and run the program"
	@echo "  clean        - Remove build artifacts"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  DB_HOST      - PostgreSQL host (default: localhost)"
	@echo "  DB_PORT      - PostgreSQL port (default: 5432)"
	@echo "  DB_USER      - PostgreSQL user (default: postgres)"
	@echo "  DB_PASSWORD  - PostgreSQL password (default: postgres)"
	@echo "  DB_NAME      - PostgreSQL database name (default: steam_reviews)"