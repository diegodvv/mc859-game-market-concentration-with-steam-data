# Steam Game Market Database Assembler

A high-performance Go application that processes Steam game review data and assembles it into a SQLite database for efficient data science analysis.

## 🚀 Overview

This tool solves the memory limitations of processing large-scale Steam review data by:
- **Memory-efficient processing**: Uses streaming techniques to handle 50M+ reviews with minimal RAM
- **Database-first approach**: Stores data in SQLite for fast querying and analysis
- **Scalable architecture**: Processes data in chunks with transaction-based commits
- **Analysis-ready output**: Generates indexed tables optimized for data science workflows

## 📊 Data Scale

- **Reviews**: 50M+ individual user reviews
- **Users**: 20M+ unique Steam users
- **Games**: 98K+ games from Steam catalog
- **Memory usage**: ~2GB peak (vs 30GB+ for in-memory approaches)
- **Processing time**: ~30-45 minutes for full dataset

## 🏗️ Database Schema

### Tables Created

| Table | Purpose | Key Fields |
|-------|---------|------------|
| `games` | Game metadata | `game_id`, `name`, `genre_ids`, `categories_ids`, pricing |
| `users` | Unique reviewers | `user_id` (Steam ID) |
| `reviews` | Review data (graph edges) | `review_id`, `user_id`, `game_id`, `voted_up`, `received_for_free` |
| `user_game_connections` | Pre-computed aggregations | `user_id`, `game_id`, `review_count`, `positive_reviews` |

### Indexes for Performance

- `idx_reviews_game_id` - Fast game-based queries
- `idx_reviews_user_id` - Fast user-based queries  
- `idx_reviews_voted_up` - Sentiment filtering
- `idx_games_genre_ids` - Genre-based analysis
- `idx_games_categories_ids` - Category filtering

## 🛠️ Installation & Setup

### Prerequisites

- Go 1.21+ 
- SQLite3 (installed automatically in devcontainer)
- ~40GB free disk space for database and processing
- 4GB+ RAM recommended

### Quick Start

```bash
# Navigate to the database assembler directory
cd graph-analyser/assemble-graph-database

# Install dependencies and build
make build

# Run the full assembly process (30-45 minutes)
make run
```

### Build Commands

```bash
make deps      # Install Go dependencies
make build     # Compile the binary
make run       # Build and execute the assembler
make clean     # Remove build artifacts and database
make db-info   # Show database statistics (requires completion)
make help      # Show all available commands
```

## 📈 Usage Example

### Running the Assembler

```bash
$ make run
🚀 Starting database-based graph assembly...
📂 Loading game data into database...
   Inserted 1000 games...
   Inserted 2000 games...
   ...
✓ Inserted 98101 games into database
🔍 Processing review files...
📈 Progress: 1000/39713 (2.5%) - Reviews: 1245678
📈 Progress: 2000/39713 (5.0%) - Reviews: 2891234
...
📊 Creating database indexes...
✅ Database assembly completed in 42m15s

📊 Database Statistics:
   Total Games: 98101
   Total Users: 22118178  
   Total Reviews: 51224528
   Positive Reviews: 35186421
   Free Game Reviews: 3847291
   Unique User-Game Pairs: 28945672
```

### Verifying the Database

```bash
# Check database file size
ls -lh steam_reviews.db

# Query basic statistics
make db-info

# Run sample queries (after completion)
sqlite3 steam_reviews.db < analysis_queries.sql
```

## 🔍 Data Science Integration

### SQL Analysis Examples

The assembler generates `analysis_queries.sql` with sample queries:

```sql
-- Top games by review count
SELECT g.name, COUNT(r.review_id) as reviews
FROM games g JOIN reviews r ON g.game_id = r.game_id 
GROUP BY g.game_id ORDER BY reviews DESC LIMIT 10;

-- Genre-based analysis
SELECT g.name, COUNT(r.review_id) as reviews,
       AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as positive_rate
FROM games g JOIN reviews r ON g.game_id = r.game_id
WHERE JSON_EXTRACT(g.genre_ids, '$') LIKE '%"1"%'  -- Action games
GROUP BY g.game_id HAVING reviews >= 100
ORDER BY positive_rate DESC;

-- User behavior analysis
SELECT user_id, COUNT(DISTINCT game_id) as games_reviewed,
       AVG(CASE WHEN voted_up THEN 1.0 ELSE 0.0 END) as positivity_rate
FROM reviews GROUP BY user_id 
HAVING games_reviewed >= 10 ORDER BY games_reviewed DESC;
```

### Python Integration

```python
import sqlite3
import pandas as pd

# Connect to the database
conn = sqlite3.connect('steam_reviews.db')

# Load data efficiently
top_games = pd.read_sql_query("""
    SELECT g.name, g.genre_ids, COUNT(r.review_id) as review_count,
           AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as positive_rate
    FROM games g JOIN reviews r ON g.game_id = r.game_id 
    GROUP BY g.game_id HAVING review_count >= 1000
    ORDER BY positive_rate DESC
""", conn)

# Genre filtering without loading full dataset
action_games = pd.read_sql_query("""
    SELECT * FROM games 
    WHERE JSON_EXTRACT(genre_ids, '$') LIKE '%"1"%'
""", conn)
```

## 🎯 Performance Optimization

### Memory Management

- **Streaming processing**: Reviews processed file-by-file, not loaded entirely into memory
- **Transaction batching**: Database writes use transactions for consistency and speed
- **Periodic cleanup**: Explicit garbage collection every 2000 files
- **Index creation**: Deferred until after data loading for optimal performance

### Processing Efficiency

- **Concurrent-safe**: Uses prepared statements for thread safety
- **Error handling**: Graceful handling of malformed JSON files
- **Progress tracking**: Real-time progress updates with ETA calculation
- **Resource monitoring**: Memory usage reporting during processing

## 🛠️ Troubleshooting

### Common Issues

**Error: "no required module provides package"**
```bash
cd graph-analyser/assemble-graph-database
go mod tidy
make build
```

**Error: "failed to open filtered_apps_dict.json"**
- Ensure you've run the steam-applist-scraper first
- Check that `../../steam-applist-scraper/filtered_apps_dict.json` exists

**Database locked errors**
```bash
make clean  # Remove existing database
make run    # Restart assembly
```

**Out of disk space**
- The final database will be ~5-10GB
- Ensure 40GB+ free space for processing
- Monitor with `df -h` during assembly

### Memory Requirements

| Dataset Size | Recommended RAM | Processing Time |
|--------------|-----------------|-----------------|
| Full (50M reviews) | 4GB+ | 30-45 minutes |
| Subset (10M reviews) | 2GB+ | 8-12 minutes |
| Sample (1M reviews) | 1GB+ | 1-2 minutes |

## 🔬 Advanced Usage

### Custom Analysis Workflows

1. **Filter by genre during assembly** (modify `loadGameDataToDB()`)
2. **Add custom aggregation tables** (extend `createIndexes()`)
3. **Export subsets to CSV** (query specific data ranges)
4. **Integration with R/Python** (direct SQLite connectivity)

### Performance Tuning

```sql
-- Optimize for read-heavy workloads
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = 10000;
PRAGMA temp_store = memory;
```

### Network Analysis Preparation

```sql
-- Create edge list for graph analysis tools (NetworkX, igraph)
CREATE VIEW game_user_edges AS
SELECT DISTINCT r1.game_id as source, r2.game_id as target, 
       COUNT(*) as weight
FROM reviews r1 JOIN reviews r2 ON r1.user_id = r2.user_id 
WHERE r1.game_id < r2.game_id 
GROUP BY r1.game_id, r2.game_id
HAVING weight >= 10;
```

## 📚 Related Projects

- **Original Go assembler**: `../assemble-graph-go/` (memory-intensive version)
- **Python analysis tools**: `../../steam_analysis.py` (visualization and metrics)
- **Steam scrapers**: `../../steam-applist-scraper/` and `../../steam-review-scraper/`

## 🤝 Contributing

1. **Memory optimizations**: Improve streaming algorithms
2. **Query performance**: Add specialized indexes
3. **Analysis functions**: Extend SQL analysis examples
4. **Documentation**: Add more usage examples

## 📄 License

Part of the MC859 Game Market Concentration research project.

---

**Next Steps**: After assembly completion, use `../../steam_analysis.py` for automated visualization and analysis, or query the database directly for custom research questions.