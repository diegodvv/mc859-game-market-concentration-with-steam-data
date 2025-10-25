# Graph Assembler - Go Version

This Go script is a port of the `assemble-graph-optimized.ipynb` Jupyter notebook. It processes Steam game and review data to build graph structures for market concentration analysis.

## Overview

The script performs the following operations:

1. **Load Game Data**: Reads filtered Steam app data from `../steam-applist-scraper/filtered_apps_dict.json`
2. **Discover Review Files**: Scans `../steam-review-scraper/data/` for review JSON files
3. **Process Reviews**: Reads each review file and builds graph data structures including:
   - Game and user node lists
   - Edge details with review metadata
   - Mapping structures connecting users, games, and reviews
4. **Export Data**: Outputs processed data in both JSON and CSV formats

## Requirements

- Go 1.21 or higher
- Input data files:
  - `../steam-applist-scraper/filtered_apps_dict.json`
  - Review files in `../steam-review-scraper/data/review_*.json`

## Usage

### Using Make (recommended)

```bash
# Build and run
make run

# Build only
make build

# Run with verbose logging
make run-verbose

# Test compilation
make test

# Clean up output files
make clean
```

### Manual compilation and execution

```bash
# Compile
go build -o assemble-graph-optimized assemble-graph-optimized.go

# Run
./assemble-graph-optimized
```

## Output Files

The script generates the following files:

### JSON Files (with "_optimized" suffix)
- `game_node_ids_optimized.json` - List of all game IDs
- `user_node_ids_optimized.json` - List of all user IDs
- `edges_details_by_review_id_optimized.json` - Review metadata indexed by review ID
- `edges_review_id_user_id_pairs_by_game_id_optimized.json` - User-review pairs grouped by game
- `edges_review_id_game_id_pairs_by_user_id_optimized.json` - Game-review pairs grouped by user

### CSV Files
- `game_node_ids.csv` - Game node IDs in tabular format
- `user_node_ids.csv` - User node IDs in tabular format
- `edges_details_by_review_id.csv` - Edge details with review metadata
- `edges_review_id_user_id_pairs_by_game_id.csv` - Game-user-review relationships
- `edges_review_id_game_id_pairs_by_user_id.csv` - User-game-review relationships

## Performance Features

- **Memory Management**: Periodic garbage collection during processing (equivalent to Python's `gc.collect()`)
- **Progress Tracking**: Shows processing progress every 100 files
- **Error Handling**: Continues processing even if individual review files fail to load
- **Efficient I/O**: Streams JSON data instead of loading everything into memory at once

## Differences from Python Version

- **Type Safety**: Go's static typing prevents runtime type errors
- **Memory Efficiency**: More explicit memory management and garbage collection
- **Error Handling**: Explicit error handling throughout the pipeline
- **Performance**: Generally faster execution due to Go's compiled nature
- **Concurrent Safety**: Ready for future parallelization if needed

## Troubleshooting

- Ensure input directories and files exist before running
- Check file permissions if you encounter access errors
- Monitor memory usage for large datasets
- Review the log output for detailed error messages