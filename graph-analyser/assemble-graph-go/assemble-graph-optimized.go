package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Data structures to mirror the Python notebook
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

type EdgeDetails struct {
	VotedUp         bool `json:"voted_up"`
	ReceivedForFree bool `json:"received_for_free"`
}

type ReviewGamePair struct {
	ReviewID string
	GameID   string
}

type ReviewUserPair struct {
	ReviewID string
	UserID   string
}

// Global data structures
var (
	gameNodeIDs                      []string
	gameDetailsByGameID              map[string]GameDetails
	userNodeIDs                      []string
	edgesDetailsByReviewID           map[string]EdgeDetails
	edgesReviewIDUserIDPairsByGameID map[string][]ReviewUserPair
	edgesReviewIDGameIDPairsByUserID map[string][]ReviewGamePair
)

func init() {
	// Initialize data structures
	gameNodeIDs = make([]string, 0)
	gameDetailsByGameID = make(map[string]GameDetails)
	userNodeIDs = make([]string, 0)
	edgesDetailsByReviewID = make(map[string]EdgeDetails)
	edgesReviewIDUserIDPairsByGameID = make(map[string][]ReviewUserPair)
	edgesReviewIDGameIDPairsByUserID = make(map[string][]ReviewGamePair)
}

func main() {
	startTime := time.Now()
	fmt.Println("🚀 Starting graph assembly...")
	fmt.Printf("📋 Go version: %s\n", runtime.Version())
	fmt.Printf("💻 Available CPU cores: %d\n", runtime.NumCPU())
	fmt.Printf("⏰ Start time: %s\n\n", startTime.Format("2006-01-02 15:04:05"))

	// Load game data
	fmt.Println("📂 Loading game data...")
	gameLoadStart := time.Now()
	if err := loadGameData(); err != nil {
		log.Fatalf("Error loading game data: %v", err)
	}
	fmt.Printf("✓ Loaded %d games from filtered apps dictionary (took %v)\n\n",
		len(gameDetailsByGameID), time.Since(gameLoadStart))

	// Find review files
	fmt.Println("🔍 Scanning for review files...")
	scanStart := time.Now()
	gameIDsWithReviewFile, err := findReviewFiles()
	if err != nil {
		log.Fatalf("Error finding review files: %v", err)
	}
	fmt.Printf("✓ Found %d review files to process (took %v)\n\n",
		len(gameIDsWithReviewFile), time.Since(scanStart))

	// Process review files
	fmt.Println("⚙️  Starting review file processing...")
	processStart := time.Now()
	if err := processReviewFiles(gameIDsWithReviewFile); err != nil {
		log.Fatalf("Error processing review files: %v", err)
	}
	processDuration := time.Since(processStart)
	fmt.Printf("✅ Processed all %d review files (took %v)\n", len(gameIDsWithReviewFile), processDuration)
	fmt.Printf("   📊 Data structures built:\n")
	fmt.Printf("      - Total game nodes: %d\n", len(gameNodeIDs))
	fmt.Printf("      - Total user nodes: %d\n", len(userNodeIDs))
	fmt.Printf("      - Total review edges: %d\n\n", len(edgesDetailsByReviewID))

	// Export to JSON
	fmt.Println("💾 Exporting data to JSON files...")
	jsonStart := time.Now()
	if err := exportToJSON(); err != nil {
		log.Fatalf("Error exporting to JSON: %v", err)
	}
	fmt.Printf("✓ Exported 5 JSON files with '_optimized' suffix (took %v)\n\n", time.Since(jsonStart))

	// Export to CSV
	fmt.Println("📄 Exporting data to CSV files...")
	csvStart := time.Now()
	if err := exportToCSV(); err != nil {
		log.Fatalf("Error exporting to CSV: %v", err)
	}
	fmt.Printf("✓ Exported 6 CSV files (took %v)\n\n", time.Since(csvStart))

	totalDuration := time.Since(startTime)
	fmt.Println("🎉 Graph assembly completed successfully!")
	fmt.Printf("⏱️  Total execution time: %v\n", totalDuration)
	fmt.Printf("📊 Final statistics:\n")
	fmt.Printf("   - Games processed: %d\n", len(gameDetailsByGameID))
	fmt.Printf("   - Users found: %d unique\n", countUniqueUsers())
	fmt.Printf("   - Reviews processed: %d\n", len(edgesDetailsByReviewID))
	fmt.Printf("   - Processing rate: %.0f reviews/second\n", float64(len(edgesDetailsByReviewID))/totalDuration.Seconds())
}

// loadGameData loads the filtered apps dictionary from JSON file
func loadGameData() error {
	file, err := os.Open("../../steam-applist-scraper/filtered_apps_dict.json")
	if err != nil {
		return fmt.Errorf("failed to open filtered_apps_dict.json: %v", err)
	}
	defer file.Close()

	var filteredAppDict map[string]GameDetails
	if err := json.NewDecoder(file).Decode(&filteredAppDict); err != nil {
		return fmt.Errorf("failed to decode filtered_apps_dict.json: %v", err)
	}

	gameDetailsByGameID = filteredAppDict

	// Convert app IDs to integers and add to game node IDs
	for appID := range filteredAppDict {
		if _, err := strconv.Atoi(appID); err == nil {
			gameNodeIDs = append(gameNodeIDs, appID)
		}
	}

	return nil
}

// findReviewFiles scans the review data directory and extracts game IDs
func findReviewFiles() ([]string, error) {
	directory := "../../steam-review-scraper/data"

	var gameIDs []string

	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasPrefix(d.Name(), "review_") && strings.HasSuffix(d.Name(), ".json") {
			// Extract game ID from filename like "review_10.json"
			name := d.Name()
			parts := strings.Split(name, "_")
			if len(parts) >= 2 {
				gameIDPart := strings.Split(parts[1], ".")[0]
				gameIDs = append(gameIDs, gameIDPart)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory %s: %v", directory, err)
	}

	// Sort the game IDs to match Python's sorted() behavior
	sort.Strings(gameIDs)

	return gameIDs, nil
}

// processReviewFiles processes each review file and builds the graph data structures
func processReviewFiles(gameIDsWithReviewFile []string) error {
	total := len(gameIDsWithReviewFile)
	reviewCount := 0
	errorCount := 0
	startTime := time.Now()

	fmt.Printf("Processing %d review files...\n", total)

	for i, appID := range gameIDsWithReviewFile {
		// Progress indicator with percentage and ETA
		if i%50 == 0 || i == total-1 {
			elapsed := time.Since(startTime)
			percentage := float64(i+1) / float64(total) * 100

			var eta string
			var avgTime string
			if i > 0 {
				// Calculate average time per file
				avgTimePerFile := elapsed / time.Duration(i+1)
				avgTime = fmt.Sprintf("%.2fs/file", avgTimePerFile.Seconds())

				// Calculate estimated remaining time
				remaining := total - (i + 1)
				estimatedRemaining := time.Duration(remaining) * avgTimePerFile
				eta = fmt.Sprintf("ETA: %v", formatDuration(estimatedRemaining))
			} else {
				avgTime = "calculating..."
				eta = "ETA: calculating..."
			}

			fmt.Printf("📈 Progress: %d/%d (%.1f%%) - App ID: %s - Reviews: %d - %s - %s\n",
				i+1, total, percentage, appID, reviewCount, avgTime, eta)
		}

		// Trigger garbage collection periodically with memory stats
		if i%1000 == 0 && i > 0 {
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("🧹 Memory cleanup at file %d - Memory usage: %.2f MB\n",
				i, float64(m.Alloc)/1024/1024)
		}

		filePath := fmt.Sprintf("../../steam-review-scraper/data/review_%s.json", appID)

		file, err := os.Open(filePath)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to open %s: %v\n", filePath, err)
			errorCount++
			continue
		}

		var reviewsData ReviewsData
		if err := json.NewDecoder(file).Decode(&reviewsData); err != nil {
			file.Close()
			fmt.Printf("⚠️  Warning: Failed to decode %s: %v\n", filePath, err)
			errorCount++
			continue
		}
		file.Close()

		fileReviewCount := len(reviewsData.Reviews)
		reviewCount += fileReviewCount

		// Log for files with many reviews
		if fileReviewCount > 10000 {
			fmt.Printf("📚 Large file: %s has %d reviews\n", appID, fileReviewCount)
		}

		// Process each review in the file
		for reviewID, review := range reviewsData.Reviews {
			userID := review.Author.SteamID

			// Add to node lists
			userNodeIDs = append(userNodeIDs, userID)
			gameNodeIDs = append(gameNodeIDs, appID)

			// Store edge details
			edgesDetailsByReviewID[reviewID] = EdgeDetails{
				VotedUp:         review.VotedUp,
				ReceivedForFree: review.ReceivedForFree,
			}

			// Build mapping structures
			if edgesReviewIDGameIDPairsByUserID[userID] == nil {
				edgesReviewIDGameIDPairsByUserID[userID] = make([]ReviewGamePair, 0)
			}
			edgesReviewIDGameIDPairsByUserID[userID] = append(
				edgesReviewIDGameIDPairsByUserID[userID],
				ReviewGamePair{ReviewID: reviewID, GameID: appID},
			)

			if edgesReviewIDUserIDPairsByGameID[appID] == nil {
				edgesReviewIDUserIDPairsByGameID[appID] = make([]ReviewUserPair, 0)
			}
			edgesReviewIDUserIDPairsByGameID[appID] = append(
				edgesReviewIDUserIDPairsByGameID[appID],
				ReviewUserPair{ReviewID: reviewID, UserID: userID},
			)
		}
	}

	if errorCount > 0 {
		fmt.Printf("⚠️  Processing completed with %d errors/warnings\n", errorCount)
	}

	fmt.Printf("✅ Review processing summary:\n")
	fmt.Printf("   - Files processed: %d/%d\n", total-errorCount, total)
	fmt.Printf("   - Total reviews: %d\n", reviewCount)
	fmt.Printf("   - Unique users: %d\n", len(edgesReviewIDGameIDPairsByUserID))
	fmt.Printf("   - Games with reviews: %d\n", len(edgesReviewIDUserIDPairsByGameID))

	return nil
}

// exportToJSON writes all data structures to JSON files with "_optimized" suffix
func exportToJSON() error {
	files := []struct {
		filename string
		data     interface{}
		desc     string
	}{
		{"game_node_ids_optimized.json", gameNodeIDs, "game node IDs"},
		{"user_node_ids_optimized.json", userNodeIDs, "user node IDs"},
		{"edges_details_by_review_id_optimized.json", edgesDetailsByReviewID, "edge details"},
		{"edges_review_id_user_id_pairs_by_game_id_optimized.json", edgesReviewIDUserIDPairsByGameID, "game-user pairs"},
		{"edges_review_id_game_id_pairs_by_user_id_optimized.json", edgesReviewIDGameIDPairsByUserID, "user-game pairs"},
	}

	for i, file := range files {
		fmt.Printf("📄 Exporting %s (%d/%d)...\n", file.desc, i+1, len(files))
		if err := writeJSONFile(file.filename, file.data); err != nil {
			return fmt.Errorf("failed to export %s: %v", file.filename, err)
		}

		// Show file size
		if info, err := os.Stat(file.filename); err == nil {
			fmt.Printf("   ✓ %s (%.2f MB)\n", file.filename, float64(info.Size())/1024/1024)
		}
	}

	return nil
}

// writeJSONFile is a helper function to write data to a JSON file
func writeJSONFile(filename string, data interface{}) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", filename, err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print JSON

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode JSON to %s: %v", filename, err)
	}

	return nil
}

// exportToCSV writes all data structures to CSV files
func exportToCSV() error {
	csvExports := []struct {
		function func() error
		desc     string
		filename string
	}{
		{writeGameNodeIdsCSV, "game node IDs", "game_node_ids.csv"},
		{writeUserNodeIdsCSV, "user node IDs", "user_node_ids.csv"},
		{writeGameDetailsCSV, "game details", "game_details.csv"},
		{writeEdgesDetailsByReviewIdCSV, "edge details", "edges_details_by_review_id.csv"},
		{writeEdgesReviewIdUserIdPairsByGameIdCSV, "game-user pairs", "edges_review_id_user_id_pairs_by_game_id.csv"},
		{writeEdgesReviewIdGameIdPairsByUserIdCSV, "user-game pairs", "edges_review_id_game_id_pairs_by_user_id.csv"},
	}

	for i, export := range csvExports {
		fmt.Printf("📊 Exporting %s CSV (%d/%d)...\n", export.desc, i+1, len(csvExports))
		if err := export.function(); err != nil {
			return fmt.Errorf("failed to export %s: %v", export.filename, err)
		}

		// Show file size and row count estimate
		if info, err := os.Stat(export.filename); err == nil {
			fmt.Printf("   ✓ %s (%.2f MB)\n", export.filename, float64(info.Size())/1024/1024)
		}
	}

	return nil
}

// writeGameNodeIdsCSV exports game node IDs to CSV
func writeGameNodeIdsCSV() error {
	file, err := os.Create("game_node_ids.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"game_node_id"}); err != nil {
		return err
	}

	// Write data
	for _, gameID := range gameNodeIDs {
		if err := writer.Write([]string{gameID}); err != nil {
			return err
		}
	}

	return nil
}

// writeUserNodeIdsCSV exports user node IDs to CSV
func writeUserNodeIdsCSV() error {
	file, err := os.Create("user_node_ids.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"user_node_id"}); err != nil {
		return err
	}

	// Write data
	for _, userID := range userNodeIDs {
		if err := writer.Write([]string{userID}); err != nil {
			return err
		}
	}

	return nil
}

func writeGameDetailsCSV() error {
	file, err := os.Create("game_details.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	headers := []string{
		"game_id",
		"name",
		"steam_appid",
		"price_currency",
		"price_final",
		"price_initial",
		"categories_ids",
		"genre_ids",
		"release_date",
		"rating_dejus_rating",
		"rating_dejus_required_age",
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	// Write data for each game
	for gameID, gameDetails := range gameDetailsByGameID {
		row := make([]string, len(headers))

		// game_id (dict key)
		row[0] = gameID

		// name
		if name, ok := gameDetails["name"].(string); ok {
			row[1] = name
		}

		// steam_appid
		if appid, ok := gameDetails["steam_appid"]; ok {
			row[2] = fmt.Sprintf("%v", appid)
		}

		// price fields from price_overview
		if priceOverview, ok := gameDetails["price_overview"].(map[string]interface{}); ok {
			if currency, ok := priceOverview["currency"].(string); ok {
				row[3] = currency
			}
			if finalPrice, ok := priceOverview["final"]; ok {
				row[4] = fmt.Sprintf("%v", finalPrice)
			}
			if initialPrice, ok := priceOverview["initial"]; ok {
				row[5] = fmt.Sprintf("%v", initialPrice)
			}
		}

		// categories_ids as JSON array
		if categories, ok := gameDetails["categories"].([]interface{}); ok {
			var categoryIDs []interface{}
			for _, category := range categories {
				if catMap, ok := category.(map[string]interface{}); ok {
					if id, ok := catMap["id"]; ok {
						categoryIDs = append(categoryIDs, id)
					}
				}
			}
			if categoriesJSON, err := json.Marshal(categoryIDs); err == nil {
				row[6] = string(categoriesJSON)
			}
		}

		// genre_ids as JSON array
		if genres, ok := gameDetails["genres"].([]interface{}); ok {
			var genreIDs []interface{}
			for _, genre := range genres {
				if genreMap, ok := genre.(map[string]interface{}); ok {
					if id, ok := genreMap["id"]; ok {
						genreIDs = append(genreIDs, id)
					}
				}
			}
			if genresJSON, err := json.Marshal(genreIDs); err == nil {
				row[7] = string(genresJSON)
			}
		}

		// release_date.date
		if releaseDate, ok := gameDetails["release_date"].(map[string]interface{}); ok {
			if date, ok := releaseDate["date"].(string); ok {
				row[8] = date
			}
		}

		// rating_dejus_rating and rating_dejus_required_age
		if ratings, ok := gameDetails["ratings"].(map[string]interface{}); ok {
			if dejus, ok := ratings["dejus"].(map[string]interface{}); ok {
				if rating, ok := dejus["rating"].(string); ok {
					row[9] = rating
				}
				if requiredAge, ok := dejus["required_age"].(string); ok {
					row[10] = requiredAge
				}
			}
		}

		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// writeEdgesDetailsByReviewIdCSV exports edge details to CSV
func writeEdgesDetailsByReviewIdCSV() error {
	file, err := os.Create("edges_details_by_review_id.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"review_id", "voted_up", "received_for_free"}); err != nil {
		return err
	}

	// Write data
	for reviewID, details := range edgesDetailsByReviewID {
		row := []string{
			reviewID,
			fmt.Sprintf("%t", details.VotedUp),
			fmt.Sprintf("%t", details.ReceivedForFree),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return nil
}

// writeEdgesReviewIdUserIdPairsByGameIdCSV exports review-user pairs by game ID to CSV
func writeEdgesReviewIdUserIdPairsByGameIdCSV() error {
	file, err := os.Create("edges_review_id_user_id_pairs_by_game_id.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"game_id", "review_id", "user_id"}); err != nil {
		return err
	}

	// Write data
	for gameID, pairs := range edgesReviewIDUserIDPairsByGameID {
		for _, pair := range pairs {
			row := []string{gameID, pair.ReviewID, pair.UserID}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeEdgesReviewIdGameIdPairsByUserIdCSV exports review-game pairs by user ID to CSV
func writeEdgesReviewIdGameIdPairsByUserIdCSV() error {
	file, err := os.Create("edges_review_id_game_id_pairs_by_user_id.csv")
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"user_id", "review_id", "game_id"}); err != nil {
		return err
	}

	// Write data
	for userID, pairs := range edgesReviewIDGameIDPairsByUserID {
		for _, pair := range pairs {
			row := []string{userID, pair.ReviewID, pair.GameID}
			if err := writer.Write(row); err != nil {
				return err
			}
		}
	}

	return nil
}

// countUniqueUsers returns the number of unique users in the dataset
func countUniqueUsers() int {
	userSet := make(map[string]bool)
	for _, userID := range userNodeIDs {
		userSet[userID] = true
	}
	return len(userSet)
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
}
