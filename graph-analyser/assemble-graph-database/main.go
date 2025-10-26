package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type GameDetails map[string]interface{}

type ReviewAuthor struct {
	SteamID string `json:"steamid"`
}

type Review struct {
	Author          ReviewAuthor `json:"author"`
	VotedUp         bool         `json:"voted_up"`
	ReceivedForFree bool         `json:"received_for_free"`
}

type ReviewsData struct {
	Reviews map[string]Review `json:"reviews"`
}

type ReviewRecord struct {
	ReviewID        string
	GameID          string
	UserID          string
	VotedUp         bool
	ReceivedForFree bool
}

var db *sql.DB

func main() {
	startTime := time.Now()
	fmt.Println("🚀 Starting database-based graph assembly...")

	// Initialize database
	if err := initializeDatabase(); err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	// Load and insert game data
	fmt.Println("📂 Loading game data into database...")
	if err := loadGameDataToDB(); err != nil {
		log.Fatalf("Error loading game data: %v", err)
	}

	// Find and process review files
	fmt.Println("🔍 Processing review files...")
	gameIDs, err := findReviewFiles()
	if err != nil {
		log.Fatalf("Error finding review files: %v", err)
	}

	if err := processReviewFilesToDB(gameIDs); err != nil {
		log.Fatalf("Error processing reviews: %v", err)
	}

	// Create indexes for better query performance
	fmt.Println("📊 Creating database indexes...")
	if err := createIndexes(); err != nil {
		log.Fatalf("Error creating indexes: %v", err)
	}

	fmt.Printf("✅ Database assembly completed in %v\n", time.Since(startTime))

	// Show statistics
	if err := showStatistics(); err != nil {
		log.Fatalf("Error showing statistics: %v", err)
	}

	// Export sample queries for analysis
	if err := createAnalysisExamples(); err != nil {
		log.Fatalf("Error creating analysis examples: %v", err)
	}
}

func initializeDatabase() error {
	var err error
	db, err = sql.Open("sqlite3", "steam_reviews.db")
	if err != nil {
		return err
	}

	// Create tables
	schema := `
	-- Games table
	CREATE TABLE IF NOT EXISTS games (
		game_id TEXT PRIMARY KEY,
		name TEXT,
		steam_appid INTEGER,
		price_currency TEXT,
		price_final INTEGER,
		price_initial INTEGER,
		categories_ids TEXT, -- JSON array
		genre_ids TEXT,     -- JSON array
		release_date TEXT,
		rating_dejus_rating TEXT,
		rating_dejus_required_age TEXT
	);

	-- Users table (only unique users who wrote reviews)
	CREATE TABLE IF NOT EXISTS users (
		user_id TEXT PRIMARY KEY
	);

	-- Reviews table (the edges of our graph)
	CREATE TABLE IF NOT EXISTS reviews (
		review_id TEXT PRIMARY KEY,
		game_id TEXT,
		user_id TEXT,
		voted_up BOOLEAN,
		received_for_free BOOLEAN,
		FOREIGN KEY (game_id) REFERENCES games(game_id),
		FOREIGN KEY (user_id) REFERENCES users(user_id)
	);

	-- Materialized view for user-game connections (for network analysis)
	CREATE TABLE IF NOT EXISTS user_game_connections (
		user_id TEXT,
		game_id TEXT,
		review_count INTEGER,
		positive_reviews INTEGER,
		free_reviews INTEGER,
		PRIMARY KEY (user_id, game_id)
	);
	`

	_, err = db.Exec(schema)
	return err
}

func loadGameDataToDB() error {
	file, err := os.Open("../../steam-applist-scraper/filtered_apps_dict.json")
	if err != nil {
		return err
	}
	defer file.Close()

	var filteredAppDict map[string]GameDetails
	if err := json.NewDecoder(file).Decode(&filteredAppDict); err != nil {
		return err
	}

	// Prepare insert statement
	stmt, err := db.Prepare(`INSERT OR REPLACE INTO games 
		(game_id, name, steam_appid, price_currency, price_final, price_initial, 
		 categories_ids, genre_ids, release_date, rating_dejus_rating, rating_dejus_required_age)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	count := 0
	for gameID, gameDetails := range filteredAppDict {
		var name, priceCurrency, releaseDate, ratingDejusRating, ratingDejusRequiredAge string
		var steamAppID, priceFinal, priceInitial interface{}
		var categoriesJSON, genresJSON string

		if val, ok := gameDetails["name"].(string); ok {
			name = val
		}
		if val, ok := gameDetails["steam_appid"]; ok {
			steamAppID = val
		}

		if priceOverview, ok := gameDetails["price_overview"].(map[string]interface{}); ok {
			if val, ok := priceOverview["currency"].(string); ok {
				priceCurrency = val
			}
			if val, ok := priceOverview["final"]; ok {
				priceFinal = val
			}
			if val, ok := priceOverview["initial"]; ok {
				priceInitial = val
			}
		}

		if categories, ok := gameDetails["categories"].([]interface{}); ok {
			var categoryIDs []interface{}
			for _, category := range categories {
				if catMap, ok := category.(map[string]interface{}); ok {
					if id, ok := catMap["id"]; ok {
						categoryIDs = append(categoryIDs, id)
					}
				}
			}
			if data, err := json.Marshal(categoryIDs); err == nil {
				categoriesJSON = string(data)
			}
		}

		if genres, ok := gameDetails["genres"].([]interface{}); ok {
			var genreIDs []interface{}
			for _, genre := range genres {
				if genreMap, ok := genre.(map[string]interface{}); ok {
					if id, ok := genreMap["id"]; ok {
						genreIDs = append(genreIDs, id)
					}
				}
			}
			if data, err := json.Marshal(genreIDs); err == nil {
				genresJSON = string(data)
			}
		}

		if releaseDateMap, ok := gameDetails["release_date"].(map[string]interface{}); ok {
			if date, ok := releaseDateMap["date"].(string); ok {
				releaseDate = date
			}
		}

		if ratings, ok := gameDetails["ratings"].(map[string]interface{}); ok {
			if dejus, ok := ratings["dejus"].(map[string]interface{}); ok {
				if rating, ok := dejus["rating"].(string); ok {
					ratingDejusRating = rating
				}
				if requiredAge, ok := dejus["required_age"].(string); ok {
					ratingDejusRequiredAge = requiredAge
				}
			}
		}

		_, err = tx.Stmt(stmt).Exec(gameID, name, steamAppID, priceCurrency, priceFinal, priceInitial,
			categoriesJSON, genresJSON, releaseDate, ratingDejusRating, ratingDejusRequiredAge)
		if err != nil {
			tx.Rollback()
			return err
		}

		count++
		if count%1000 == 0 {
			fmt.Printf("   Inserted %d games...\n", count)
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	fmt.Printf("✓ Inserted %d games into database\n", count)
	return nil
}

func findReviewFiles() ([]string, error) {
	directory := "../../steam-review-scraper/data"
	var gameIDs []string

	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasPrefix(d.Name(), "review_") && strings.HasSuffix(d.Name(), ".json") {
			name := d.Name()
			parts := strings.Split(name, "_")
			if len(parts) >= 2 {
				gameIDPart := strings.Split(parts[1], ".")[0]
				gameIDs = append(gameIDs, gameIDPart)
			}
		}
		return nil
	})

	return gameIDs, err
}

func processReviewFilesToDB(gameIDs []string) error {
	const batchSize = 1000000

	// Batches for users and reviews
	userBatch := make(map[string]bool) // Using map to avoid duplicates
	reviewBatch := make([]ReviewRecord, 0, batchSize)

	total := len(gameIDs)
	reviewCount := 0
	startTime := time.Now()

	// Function to insert current batches
	insertBatches := func() error {
		if len(userBatch) > 0 || len(reviewBatch) > 0 {
			batchStart := time.Now()
			tx, err := db.Begin()
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			userCount := len(userBatch)
			reviewBatchCount := len(reviewBatch)

			// Insert users batch
			if len(userBatch) > 0 {
				userValues := make([]string, 0, len(userBatch))
				userArgs := make([]interface{}, 0, len(userBatch))

				for userID := range userBatch {
					userValues = append(userValues, "(?)")
					userArgs = append(userArgs, userID)
				}

				userQuery := fmt.Sprintf("INSERT OR IGNORE INTO users (user_id) VALUES %s",
					strings.Join(userValues, ","))

				_, err = tx.Exec(userQuery, userArgs...)
				if err != nil {
					return fmt.Errorf("error inserting users batch: %v", err)
				}
			}

			// Insert reviews batch in chunks to avoid variable limit
			if len(reviewBatch) > 0 {
				// Process reviews in smaller chunks to stay within SQLite limits
				chunkSize := 50 // Each chunk uses 50 * 5 = 250 variables (well under 999 limit)

				for i := 0; i < len(reviewBatch); i += chunkSize {
					end := i + chunkSize
					if end > len(reviewBatch) {
						end = len(reviewBatch)
					}

					chunk := reviewBatch[i:end]
					reviewValues := make([]string, 0, len(chunk))
					reviewArgs := make([]interface{}, 0, len(chunk)*5)

					for _, review := range chunk {
						reviewValues = append(reviewValues, "(?, ?, ?, ?, ?)")
						reviewArgs = append(reviewArgs, review.ReviewID, review.GameID,
							review.UserID, review.VotedUp, review.ReceivedForFree)
					}

					reviewQuery := fmt.Sprintf(`INSERT OR REPLACE INTO reviews 
						(review_id, game_id, user_id, voted_up, received_for_free) VALUES %s`,
						strings.Join(reviewValues, ","))

					_, err = tx.Exec(reviewQuery, reviewArgs...)
					if err != nil {
						return fmt.Errorf("error inserting reviews chunk: %v", err)
					}
				}
			}

			err = tx.Commit()
			if err != nil {
				return fmt.Errorf("error committing batch transaction: %v", err)
			}

			batchDuration := time.Since(batchStart)
			if reviewBatchCount > 0 {
				fmt.Printf("   💾 Batch inserted: %d users, %d reviews (%.2fs)\n",
					userCount, reviewBatchCount, batchDuration.Seconds())
			}

			// Clear batches
			userBatch = make(map[string]bool)
			reviewBatch = reviewBatch[:0]
		}
		return nil
	}

	for i, appID := range gameIDs {
		if i%500 == 0 || i == total-1 {
			percentage := float64(i+1) / float64(total) * 100
			elapsed := time.Since(startTime)

			var eta time.Duration
			if i > 0 {
				avgTimePerFile := elapsed / time.Duration(i+1)
				remainingFiles := total - (i + 1)
				eta = avgTimePerFile * time.Duration(remainingFiles)
			}

			if eta > 0 {
				fmt.Printf("📈 Progress: %d/%d (%.1f%%) - Reviews: %d - ETA: %v\n",
					i+1, total, percentage, reviewCount, eta.Round(time.Second))
			} else {
				fmt.Printf("📈 Progress: %d/%d (%.1f%%) - Reviews: %d\n",
					i+1, total, percentage, reviewCount)
			}
		}

		if i%2000 == 0 && i > 0 {
			runtime.GC()
		}

		filePath := fmt.Sprintf("../../steam-review-scraper/data/review_%s.json", appID)
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}

		var reviewsData ReviewsData
		if err := json.NewDecoder(file).Decode(&reviewsData); err != nil {
			file.Close()
			continue
		}
		file.Close()

		// Add reviews to batch
		for reviewID, review := range reviewsData.Reviews {
			userID := review.Author.SteamID

			// Add user to batch (map automatically handles duplicates)
			userBatch[userID] = true

			// Add review to batch
			reviewBatch = append(reviewBatch, ReviewRecord{
				ReviewID:        reviewID,
				GameID:          appID,
				UserID:          userID,
				VotedUp:         review.VotedUp,
				ReceivedForFree: review.ReceivedForFree,
			})

			reviewCount++

			// Insert batch when it reaches the batch size
			if len(reviewBatch) >= batchSize {
				if err := insertBatches(); err != nil {
					return err
				}
			}
		}
	}

	// Insert any remaining items in the batches
	if err := insertBatches(); err != nil {
		return err
	}

	fmt.Printf("✅ Inserted %d reviews into database\n", reviewCount)
	return nil
}

func createIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_reviews_game_id ON reviews(game_id)",
		"CREATE INDEX IF NOT EXISTS idx_reviews_user_id ON reviews(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_reviews_voted_up ON reviews(voted_up)",
		"CREATE INDEX IF NOT EXISTS idx_games_genre_ids ON games(genre_ids)",
		"CREATE INDEX IF NOT EXISTS idx_games_categories_ids ON games(categories_ids)",
	}

	for _, index := range indexes {
		if _, err := db.Exec(index); err != nil {
			return err
		}
	}

	// Create materialized view for user-game connections
	materializeQuery := `
	INSERT OR REPLACE INTO user_game_connections (user_id, game_id, review_count, positive_reviews, free_reviews)
	SELECT 
		user_id,
		game_id,
		COUNT(*) as review_count,
		SUM(CASE WHEN voted_up THEN 1 ELSE 0 END) as positive_reviews,
		SUM(CASE WHEN received_for_free THEN 1 ELSE 0 END) as free_reviews
	FROM reviews 
	GROUP BY user_id, game_id
	`

	_, err := db.Exec(materializeQuery)
	return err
}

func showStatistics() error {
	queries := map[string]string{
		"Total Games":            "SELECT COUNT(*) FROM games",
		"Total Users":            "SELECT COUNT(*) FROM users",
		"Total Reviews":          "SELECT COUNT(*) FROM reviews",
		"Positive Reviews":       "SELECT COUNT(*) FROM reviews WHERE voted_up = 1",
		"Free Game Reviews":      "SELECT COUNT(*) FROM reviews WHERE received_for_free = 1",
		"Unique User-Game Pairs": "SELECT COUNT(*) FROM user_game_connections",
	}

	fmt.Println("\n📊 Database Statistics:")
	for label, query := range queries {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			return err
		}
		fmt.Printf("   %s: %d\n", label, count)
	}

	return nil
}

func createAnalysisExamples() error {
	examples := `-- Sample SQL queries for data science analysis

-- 1. Top 10 games by review count
SELECT g.name, g.game_id, COUNT(r.review_id) as review_count
FROM games g 
JOIN reviews r ON g.game_id = r.game_id 
GROUP BY g.game_id, g.name 
ORDER BY review_count DESC 
LIMIT 10;

-- 2. Users who reviewed the most games (potential influential reviewers)
SELECT user_id, COUNT(DISTINCT game_id) as games_reviewed
FROM reviews 
GROUP BY user_id 
ORDER BY games_reviewed DESC 
LIMIT 10;

-- 3. Games with highest positive review percentage (min 100 reviews)
SELECT g.name, g.game_id,
       COUNT(r.review_id) as total_reviews,
       AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as positive_rate
FROM games g 
JOIN reviews r ON g.game_id = r.game_id 
GROUP BY g.game_id, g.name 
HAVING total_reviews >= 100
ORDER BY positive_rate DESC 
LIMIT 10;

-- 4. Genre analysis - games count by genre (you'll need to parse JSON)
SELECT genre_ids, COUNT(*) as game_count
FROM games 
WHERE genre_ids IS NOT NULL AND genre_ids != ''
GROUP BY genre_ids 
ORDER BY game_count DESC;

-- 5. User similarity - users who reviewed similar games (for recommendation systems)
WITH user_games AS (
    SELECT user_id, GROUP_CONCAT(game_id) as games
    FROM reviews 
    WHERE user_id IN (SELECT user_id FROM reviews GROUP BY user_id HAVING COUNT(*) BETWEEN 10 AND 100)
    GROUP BY user_id
)
SELECT COUNT(*) as similar_users_pairs
FROM user_games u1 
JOIN user_games u2 ON u1.user_id < u2.user_id;

-- 6. Game network analysis - games often reviewed by same users
SELECT r1.game_id as game1, r2.game_id as game2, COUNT(*) as shared_users
FROM reviews r1 
JOIN reviews r2 ON r1.user_id = r2.user_id AND r1.game_id < r2.game_id
GROUP BY r1.game_id, r2.game_id 
HAVING shared_users >= 10
ORDER BY shared_users DESC;

-- 7. Filter games by specific genre and analyze
-- First, you need to extract genre IDs from JSON. For SQLite with JSON support:
SELECT g.name, COUNT(r.review_id) as reviews,
       AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as positive_rate
FROM games g
JOIN reviews r ON g.game_id = r.game_id
WHERE JSON_EXTRACT(g.genre_ids, '$') LIKE '%"1"%'  -- Replace "1" with actual genre ID
GROUP BY g.game_id, g.name
HAVING reviews >= 50
ORDER BY positive_rate DESC;
`

	return os.WriteFile("analysis_queries.sql", []byte(examples), 0644)
}
