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

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Get database connection details from environment variables
	dbHost := getEnvWithDefault("DB_HOST", "localhost")
	dbPort := getEnvWithDefault("DB_PORT", "5432")
	dbUser := getEnvWithDefault("DB_USER", "postgres")
	dbPassword := getEnvWithDefault("DB_PASSWORD", "postgres")
	dbName := getEnvWithDefault("DB_NAME", "steam_reviews")
	dbSSLMode := getEnvWithDefault("DB_SSLMODE", "disable")

	// PostgreSQL connection string
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, dbSSLMode)

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	fmt.Printf("✓ Connected to PostgreSQL database: %s@%s:%s/%s\n", dbUser, dbHost, dbPort, dbName)

	// Create tables (PostgreSQL syntax)
	schema := `
	-- Games table
	CREATE TABLE IF NOT EXISTS games (
		game_id VARCHAR(255) PRIMARY KEY,
		name TEXT,
		steam_appid INTEGER,
		price_currency VARCHAR(10),
		price_final INTEGER,
		price_initial INTEGER,
		categories_ids JSONB, -- JSON array
		genre_ids JSONB,     -- JSON array
		release_date TEXT,
		rating_dejus_rating TEXT,
		rating_dejus_required_age TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Users table (only unique users who wrote reviews)
	CREATE TABLE IF NOT EXISTS users (
		user_id VARCHAR(255) PRIMARY KEY,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Reviews table (the edges of our graph)
	CREATE TABLE IF NOT EXISTS reviews (
		review_id VARCHAR(255) PRIMARY KEY,
		game_id VARCHAR(255) NOT NULL,
		user_id VARCHAR(255) NOT NULL,
		voted_up BOOLEAN NOT NULL,
		received_for_free BOOLEAN NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (game_id) REFERENCES games(game_id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
	);

	-- Materialized view for user-game connections (for network analysis)
	CREATE TABLE IF NOT EXISTS user_game_connections (
		user_id VARCHAR(255),
		game_id VARCHAR(255),
		review_count INTEGER NOT NULL DEFAULT 0,
		positive_reviews INTEGER NOT NULL DEFAULT 0,
		free_reviews INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (user_id, game_id),
		FOREIGN KEY (game_id) REFERENCES games(game_id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
	);
	`

	_, err = db.Exec(schema)
	return err
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
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

	// Prepare insert statement (PostgreSQL UPSERT syntax)
	stmt, err := db.Prepare(`INSERT INTO games 
		(game_id, name, steam_appid, price_currency, price_final, price_initial, 
		 categories_ids, genre_ids, release_date, rating_dejus_rating, rating_dejus_required_age)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (game_id) DO UPDATE SET
		name = EXCLUDED.name,
		steam_appid = EXCLUDED.steam_appid,
		price_currency = EXCLUDED.price_currency,
		price_final = EXCLUDED.price_final,
		price_initial = EXCLUDED.price_initial,
		categories_ids = EXCLUDED.categories_ids,
		genre_ids = EXCLUDED.genre_ids,
		release_date = EXCLUDED.release_date,
		rating_dejus_rating = EXCLUDED.rating_dejus_rating,
		rating_dejus_required_age = EXCLUDED.rating_dejus_required_age`)
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
		} else {
			categoriesJSON = "null"
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
		} else {
			genresJSON = "null"
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

		// Convert empty JSON strings to null for PostgreSQL
		var categoriesJSONParam, genresJSONParam interface{}
		if categoriesJSON == "" {
			categoriesJSONParam = nil
		} else {
			categoriesJSONParam = categoriesJSON
		}
		if genresJSON == "" {
			genresJSONParam = nil
		} else {
			genresJSONParam = genresJSON
		}

		_, err = tx.Stmt(stmt).Exec(gameID, name, steamAppID, priceCurrency, priceFinal, priceInitial,
			categoriesJSONParam, genresJSONParam, releaseDate, ratingDejusRating, ratingDejusRequiredAge)
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
	const batchSize = 10000

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

			userCount := len(userBatch)
			reviewBatchCount := len(reviewBatch)

			// Try batch insert first
			err := func() error {
				tx, err := db.Begin()
				if err != nil {
					return err
				}
				defer func() {
					if err != nil {
						tx.Rollback()
					}
				}()

				// Insert users batch
				if len(userBatch) > 0 {
					userValues := make([]string, 0, len(userBatch))
					userArgs := make([]interface{}, 0, len(userBatch))
					argIndex := 1

					for userID := range userBatch {
						userValues = append(userValues, fmt.Sprintf("($%d)", argIndex))
						userArgs = append(userArgs, userID)
						argIndex++
					}

					userQuery := fmt.Sprintf("INSERT INTO users (user_id) VALUES %s ON CONFLICT (user_id) DO NOTHING",
						strings.Join(userValues, ","))

					_, err = tx.Exec(userQuery, userArgs...)
					if err != nil {
						return fmt.Errorf("error inserting users batch: %v", err)
					}
				}

				// Insert reviews batch in chunks
				if len(reviewBatch) > 0 {
					// Process reviews in smaller chunks for better performance
					chunkSize := 1000 // PostgreSQL can handle larger batches than SQLite

					for i := 0; i < len(reviewBatch); i += chunkSize {
						end := i + chunkSize
						if end > len(reviewBatch) {
							end = len(reviewBatch)
						}

						chunk := reviewBatch[i:end]
						reviewValues := make([]string, 0, len(chunk))
						reviewArgs := make([]interface{}, 0, len(chunk)*5)
						argIndex := 1

						for _, review := range chunk {
							reviewValues = append(reviewValues, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d)",
								argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4))
							reviewArgs = append(reviewArgs, review.ReviewID, review.GameID,
								review.UserID, review.VotedUp, review.ReceivedForFree)
							argIndex += 5
						}

						reviewQuery := fmt.Sprintf(`INSERT INTO reviews 
							(review_id, game_id, user_id, voted_up, received_for_free) VALUES %s
							ON CONFLICT (review_id) DO UPDATE SET
							game_id = EXCLUDED.game_id,
							user_id = EXCLUDED.user_id,
							voted_up = EXCLUDED.voted_up,
							received_for_free = EXCLUDED.received_for_free`,
							strings.Join(reviewValues, ","))

						_, err = tx.Exec(reviewQuery, reviewArgs...)
						if err != nil {
							return fmt.Errorf("error inserting reviews chunk: %v", err)
						}
					}
				}

				return tx.Commit()
			}()

			// If batch insert failed, fallback to individual inserts
			if err != nil {
				fmt.Printf("   ⚠️ Batch insert failed (%v), falling back to individual inserts...\n", err)

				// Insert users one by one
				userStmt, err := db.Prepare("INSERT INTO users (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING")
				if err != nil {
					return fmt.Errorf("error preparing user statement: %v", err)
				}
				defer userStmt.Close()

				for userID := range userBatch {
					if _, err := userStmt.Exec(userID); err != nil {
						fmt.Printf("   ⚠️ Failed to insert user %s: %v\n", userID, err)
					}
				}

				// Insert reviews one by one
				reviewStmt, err := db.Prepare(`INSERT INTO reviews 
					(review_id, game_id, user_id, voted_up, received_for_free) VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT (review_id) DO UPDATE SET
					game_id = EXCLUDED.game_id,
					user_id = EXCLUDED.user_id,
					voted_up = EXCLUDED.voted_up,
					received_for_free = EXCLUDED.received_for_free`)
				if err != nil {
					return fmt.Errorf("error preparing review statement: %v", err)
				}
				defer reviewStmt.Close()

				for _, review := range reviewBatch {
					if _, err := reviewStmt.Exec(review.ReviewID, review.GameID, review.UserID, review.VotedUp, review.ReceivedForFree); err != nil {
						fmt.Printf("   ⚠️ Failed to insert review %s: %v\n", review.ReviewID, err)
					}
				}
			}

			batchDuration := time.Since(batchStart)
			if reviewBatchCount > 0 {
				fmt.Printf("   💾 Batch processed: %d users, %d reviews (%.2fs)\n",
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
	INSERT INTO user_game_connections (user_id, game_id, review_count, positive_reviews, free_reviews)
	SELECT 
		user_id,
		game_id,
		COUNT(*) as review_count,
		SUM(CASE WHEN voted_up THEN 1 ELSE 0 END) as positive_reviews,
		SUM(CASE WHEN received_for_free THEN 1 ELSE 0 END) as free_reviews
	FROM reviews 
	GROUP BY user_id, game_id
	ON CONFLICT (user_id, game_id) DO UPDATE SET
		review_count = EXCLUDED.review_count,
		positive_reviews = EXCLUDED.positive_reviews,
		free_reviews = EXCLUDED.free_reviews
	`

	_, err := db.Exec(materializeQuery)
	return err
}

func showStatistics() error {
	queries := map[string]string{
		"Total Games":            "SELECT COUNT(*) FROM games",
		"Total Users":            "SELECT COUNT(*) FROM users",
		"Total Reviews":          "SELECT COUNT(*) FROM reviews",
		"Positive Reviews":       "SELECT COUNT(*) FROM reviews WHERE voted_up = true",
		"Free Game Reviews":      "SELECT COUNT(*) FROM reviews WHERE received_for_free = true",
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
	examples := `-- Sample SQL queries for data science analysis (PostgreSQL syntax)

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
HAVING COUNT(r.review_id) >= 100
ORDER BY positive_rate DESC 
LIMIT 10;

-- 4. Genre analysis - games count by genre (PostgreSQL JSON operations)
SELECT genre_ids, COUNT(*) as game_count
FROM games 
WHERE genre_ids IS NOT NULL 
GROUP BY genre_ids 
ORDER BY game_count DESC;

-- 5. User similarity - users who reviewed similar games (for recommendation systems)
WITH user_games AS (
    SELECT user_id, STRING_AGG(game_id, ',' ORDER BY game_id) as games
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
HAVING COUNT(*) >= 10
ORDER BY shared_users DESC;

-- 7. Filter games by specific genre and analyze (PostgreSQL JSONB operations)
SELECT g.name, COUNT(r.review_id) as reviews,
       AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as positive_rate
FROM games g
JOIN reviews r ON g.game_id = r.game_id
WHERE g.genre_ids ? '1'  -- Check if genre_ids JSONB array contains '1'
GROUP BY g.game_id, g.name
HAVING COUNT(r.review_id) >= 50
ORDER BY positive_rate DESC;

-- 8. Advanced genre analysis using JSONB functions
SELECT 
    jsonb_array_elements_text(genre_ids) as genre_id,
    COUNT(DISTINCT g.game_id) as games_count,
    AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as avg_positive_rate
FROM games g
JOIN reviews r ON g.game_id = r.game_id
WHERE genre_ids IS NOT NULL
GROUP BY genre_id
ORDER BY games_count DESC;

-- 9. Time-based analysis (using created_at timestamps)
SELECT 
    DATE_TRUNC('month', created_at) as month,
    COUNT(*) as reviews_count
FROM reviews
GROUP BY month
ORDER BY month;

-- 10. Market concentration analysis using window functions
SELECT 
    game_id,
    review_count,
    SUM(review_count) OVER() as total_reviews,
    ROUND(100.0 * review_count / SUM(review_count) OVER(), 2) as market_share,
    ROW_NUMBER() OVER(ORDER BY review_count DESC) as rank
FROM (
    SELECT game_id, COUNT(*) as review_count
    FROM reviews
    GROUP BY game_id
) ranked_games
ORDER BY review_count DESC;
`

	return os.WriteFile("analysis_queries.sql", []byte(examples), 0644)
}
