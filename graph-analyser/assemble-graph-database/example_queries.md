```sql
-- Sample SQL queries for data science analysis (PostgreSQL syntax)

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
```