# Game market concentration analysis with Steam data

This repository contains files to download data of games and it's reviews from Steam (through it's official API) and files to use that data to assemble a graph of games and users where and edge from an user to a game exists if the user has made a review about that game.

## How to use

With Python 3, install the requirements in requirements.txt, and then

### 1. Scrape the app list with details
Go to the `steam-applist-scraper` folder and run the `steam-applist-scraper.py` script.

After it's done running, run the export-steam-applist.ipynb to export the data to the `filtered_apps_dict.json` and `filtered_apps_ids.json` files.

### 2. Scrape the reviews of each app

Go to the `steam-review-scraper` folder and run the `steam-review-downloader.ipynb` notebook. It contains code to download reviews for all games whose ids were obtained above.


### 3. Assemble the graph and do some analysis on it

**Option A: PostgreSQL Database Approach (Recommended for large datasets)**

For memory-efficient processing of large-scale data (50M+ reviews) using PostgreSQL:

#### Setup PostgreSQL Database

`cd graph-analyser && docker compose --env-file assemble-graph-database/.env up -d --force-recreate`

2. **Configure and run the assembler**:
   ```bash
   cd graph-analyser/assemble-graph-database
   
   # Process data into PostgreSQL database (30-45 minutes)
   make run
   ```

This creates a `steam_reviews` PostgreSQL database optimized for data science analysis with proper indexing and JSONB support for genre/category data.

Finally, use the `analyse-graph.ipynb` file to compute some statistics of the graph and see some visualizations of the node degree distribution and the concentration of reviews per game with a Lorenz curve plot and the Gini coefficient.

### 4. Data Science Analysis

**PostgreSQL-based Analysis Pipeline (Recommended):**

1. **Install Python dependencies:**
   ```bash
   pip install -r requirements.txt
   ```

2. **Run the comprehensive analysis notebook:**
   ```bash
   # Start Jupyter
   jupyter lab
   
   # Open and run steam_market_analysis.ipynb
   # This notebook now connects to PostgreSQL and provides:
   # - Interactive visualizations and market insights
   # - Genre-based filtering and analysis using JSONB operations
   # - User behavior and game popularity metrics
   # - Market concentration analysis with Gini coefficients
   # - Custom SQL queries for advanced analysis
   ```

## Database Configuration

### PostgreSQL Environment Variables

Create a `.env` file in `graph-analyser/assemble-graph-database/` with your database configuration:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=steam_reviews
DB_SSLMODE=disable
```

## Data

The data generated from the `assemble-graph.ipynb` is also available on `https://huggingface.co/datasets/diegodvv/steam-games-reviews-and-users-graph/tree/main`
