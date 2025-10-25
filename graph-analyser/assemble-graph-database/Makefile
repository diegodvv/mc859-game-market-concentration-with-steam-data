# Makefile for database-based graph assembler

.PHONY: build run clean deps

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
	rm -f steam_reviews.db
	rm -f analysis_queries.sql

# Show database info (requires sqlite3 command)
db-info: steam_reviews.db
	@echo "Database file size:"
	@ls -lh steam_reviews.db
	@echo "\nTable statistics:"
	@sqlite3 steam_reviews.db "SELECT 'games' as table_name, COUNT(*) as count FROM games UNION ALL SELECT 'users', COUNT(*) FROM users UNION ALL SELECT 'reviews', COUNT(*) FROM reviews;"

help:
	@echo "Available targets:"
	@echo "  deps      - Install Go dependencies" 
	@echo "  build     - Compile the Go program"
	@echo "  run       - Build and run the program"
	@echo "  clean     - Remove build artifacts and database"
	@echo "  db-info   - Show database statistics (requires sqlite3)"
	@echo "  help      - Show this help message"