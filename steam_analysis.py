#!/usr/bin/env python3
"""
Steam Game Market Analysis Script
=================================

This script demonstrates how to perform data science analysis on the Steam game review data.
It works with both CSV files and SQLite database formats.

Features:
- Load and analyze large datasets efficiently using pandas and dask
- Generate plots and metrics for the game market
- Filter data by genre and regenerate analysis
- Memory-efficient processing techniques
"""

import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
import seaborn as sns
import sqlite3
import json
import warnings
from pathlib import Path
import dask.dataframe as dd
from typing import List, Dict, Tuple, Optional

warnings.filterwarnings('ignore')


class SteamAnalyzer:
    """Handles Steam game market analysis with memory efficiency"""

    def __init__(self, data_source: str = "database", db_path: str = None):
        """
        Initialize analyzer

        Args:
            data_source: "database" or "csv"
            db_path: Path to SQLite database (if using database source)
        """
        self.data_source = data_source
        self.db_path = db_path or "steam_reviews.db"
        self.conn = None

        if data_source == "database":
            self._connect_database()

    def _connect_database(self):
        """Connect to SQLite database"""
        try:
            self.conn = sqlite3.connect(self.db_path)
            print(f"✓ Connected to database: {self.db_path}")
        except Exception as e:
            print(f"❌ Database connection failed: {e}")
            raise

    def get_basic_stats(self) -> Dict:
        """Get basic statistics about the dataset"""
        if self.data_source == "database":
            return self._get_stats_from_db()
        else:
            return self._get_stats_from_csv()

    def _get_stats_from_db(self) -> Dict:
        """Get statistics from database"""
        queries = {
            'total_games': "SELECT COUNT(*) FROM games",
            'total_users': "SELECT COUNT(*) FROM users",
            'total_reviews': "SELECT COUNT(*) FROM reviews",
            'positive_reviews': "SELECT COUNT(*) FROM reviews WHERE voted_up = 1",
            'free_game_reviews': "SELECT COUNT(*) FROM reviews WHERE received_for_free = 1"
        }

        stats = {}
        for key, query in queries.items():
            stats[key] = pd.read_sql_query(query, self.conn).iloc[0, 0]

        return stats

    def _get_stats_from_csv(self) -> Dict:
        """Get statistics from CSV files using chunked reading"""
        print("📊 Analyzing CSV files (this may take a moment)...")

        # Use dask for memory-efficient processing
        try:
            reviews_df = dd.read_csv("edges_details_by_review_id.csv")
            game_user_pairs = dd.read_csv(
                "edges_review_id_user_id_pairs_by_game_id.csv")

            stats = {
                'total_reviews': len(reviews_df),
                'positive_reviews': (reviews_df['voted_up'] == True).sum().compute(),
                'free_game_reviews': (reviews_df['received_for_free'] == True).sum().compute(),
                'total_games': game_user_pairs['game_id'].nunique().compute(),
                'total_users': game_user_pairs['user_id'].nunique().compute()
            }

            return stats
        except Exception as e:
            print(f"❌ Error reading CSV files: {e}")
            return {}

    def analyze_top_games(self, limit: int = 20) -> pd.DataFrame:
        """Analyze top games by review count and rating"""
        if self.data_source == "database":
            query = """
            SELECT g.name, g.game_id, 
                   COUNT(r.review_id) as review_count,
                   AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as positive_rate,
                   g.genre_ids
            FROM games g 
            JOIN reviews r ON g.game_id = r.game_id 
            GROUP BY g.game_id, g.name, g.genre_ids
            HAVING review_count >= 100
            ORDER BY review_count DESC 
            LIMIT ?
            """
            return pd.read_sql_query(query, self.conn, params=[limit])
        else:
            # For CSV files, use chunked processing
            return self._analyze_top_games_csv(limit)

    def _analyze_top_games_csv(self, limit: int) -> pd.DataFrame:
        """Analyze top games from CSV files"""
        print("📈 Processing CSV data for top games analysis...")

        # Read data in chunks to manage memory
        chunk_size = 100000
        game_stats = {}

        for chunk in pd.read_csv("edges_review_id_user_id_pairs_by_game_id.csv", chunksize=chunk_size):
            for game_id in chunk['game_id'].unique():
                if game_id not in game_stats:
                    game_stats[game_id] = {'count': 0, 'positive': 0}

                game_reviews = chunk[chunk['game_id'] == game_id]
                game_stats[game_id]['count'] += len(game_reviews)

        # Convert to DataFrame and sort
        games_df = pd.DataFrame([
            {'game_id': game_id, 'review_count': stats['count']}
            for game_id, stats in game_stats.items()
        ]).sort_values('review_count', ascending=False).head(limit)

        return games_df

    def analyze_by_genre(self, genre_id: str) -> Dict:
        """Analyze games filtered by specific genre"""
        if self.data_source == "database":
            query = """
            SELECT g.name, g.game_id, 
                   COUNT(r.review_id) as review_count,
                   AVG(CASE WHEN r.voted_up THEN 1.0 ELSE 0.0 END) as positive_rate
            FROM games g
            JOIN reviews r ON g.game_id = r.game_id
            WHERE g.genre_ids LIKE ?
            GROUP BY g.game_id, g.name
            HAVING review_count >= 50
            ORDER BY positive_rate DESC
            LIMIT 20
            """

            df = pd.read_sql_query(
                query, self.conn, params=[f'%"{genre_id}"%'])

            return {
                'top_games': df,
                'total_games': len(df),
                'avg_positive_rate': df['positive_rate'].mean() if len(df) > 0 else 0
            }
        else:
            print("❌ Genre analysis requires database format for efficient JSON querying")
            return {}

    def create_visualizations(self, output_dir: str = "analysis_plots"):
        """Create visualization plots"""
        output_path = Path(output_dir)
        output_path.mkdir(exist_ok=True)

        # Set style
        plt.style.use('seaborn-v0_8')

        print(f"📊 Creating visualizations in {output_dir}/...")

        # 1. Basic statistics plot
        stats = self.get_basic_stats()
        if stats:
            self._plot_basic_stats(stats, output_path)

        # 2. Top games analysis
        top_games = self.analyze_top_games()
        if not top_games.empty:
            self._plot_top_games(top_games, output_path)

        # 3. User activity distribution (if using database)
        if self.data_source == "database":
            self._plot_user_activity_distribution(output_path)

        print("✓ Visualizations completed!")

    def _plot_basic_stats(self, stats: Dict, output_path: Path):
        """Plot basic dataset statistics"""
        fig, ((ax1, ax2), (ax3, ax4)) = plt.subplots(2, 2, figsize=(15, 10))
        fig.suptitle('Steam Dataset Overview', fontsize=16)

        # Total counts
        counts = [stats['total_games'],
                  stats['total_users'], stats['total_reviews']]
        labels = ['Games', 'Users', 'Reviews']
        colors = ['#1f77b4', '#ff7f0e', '#2ca02c']

        ax1.bar(labels, counts, color=colors)
        ax1.set_title('Dataset Size')
        ax1.set_ylabel('Count (millions)')
        ax1.tick_params(axis='x', rotation=45)

        # Format y-axis to show in millions
        ax1.yaxis.set_major_formatter(
            plt.FuncFormatter(lambda x, p: f'{x/1e6:.1f}M'))

        # Review sentiment
        positive_pct = (stats['positive_reviews'] /
                        stats['total_reviews']) * 100
        negative_pct = 100 - positive_pct

        ax2.pie([positive_pct, negative_pct], labels=['Positive', 'Negative'],
                colors=['#2ca02c', '#d62728'], autopct='%1.1f%%', startangle=90)
        ax2.set_title('Review Sentiment Distribution')

        # Free vs Paid reviews
        free_pct = (stats['free_game_reviews'] / stats['total_reviews']) * 100
        paid_pct = 100 - free_pct

        ax3.pie([free_pct, paid_pct], labels=['Free Games', 'Paid Games'],
                colors=['#ff7f0e', '#1f77b4'], autopct='%1.1f%%', startangle=90)
        ax3.set_title('Free vs Paid Game Reviews')

        # Reviews per game distribution (approximation)
        avg_reviews_per_game = stats['total_reviews'] / stats['total_games']
        ax4.bar(['Average Reviews per Game'], [
                avg_reviews_per_game], color='#9467bd')
        ax4.set_title('Average Reviews per Game')
        ax4.set_ylabel('Review Count')

        plt.tight_layout()
        plt.savefig(output_path / 'dataset_overview.png',
                    dpi=300, bbox_inches='tight')
        plt.close()

    def _plot_top_games(self, df: pd.DataFrame, output_path: Path):
        """Plot top games by reviews"""
        fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(20, 8))

        # Top games by review count
        top_10 = df.head(10)
        ax1.barh(range(len(top_10)), top_10['review_count'])
        ax1.set_yticks(range(len(top_10)))
        ax1.set_yticklabels(
            top_10['name'] if 'name' in top_10.columns else top_10['game_id'])
        ax1.set_xlabel('Review Count')
        ax1.set_title('Top 10 Games by Review Count')
        ax1.invert_yaxis()

        # Review count distribution
        if 'positive_rate' in df.columns:
            ax2.scatter(df['review_count'], df['positive_rate'], alpha=0.6)
            ax2.set_xlabel('Review Count')
            ax2.set_ylabel('Positive Review Rate')
            ax2.set_title('Review Count vs Positive Rate')
            ax2.set_xscale('log')

        plt.tight_layout()
        plt.savefig(output_path / 'top_games_analysis.png',
                    dpi=300, bbox_inches='tight')
        plt.close()

    def _plot_user_activity_distribution(self, output_path: Path):
        """Plot user activity distribution"""
        query = """
        SELECT COUNT(DISTINCT game_id) as games_reviewed, COUNT(*) as user_count
        FROM reviews 
        GROUP BY user_id
        HAVING games_reviewed <= 50
        """

        df = pd.read_sql_query(query, self.conn)
        activity_dist = df.groupby('games_reviewed')[
            'user_count'].sum().reset_index()

        plt.figure(figsize=(12, 6))
        plt.bar(activity_dist['games_reviewed'], activity_dist['user_count'])
        plt.xlabel('Number of Games Reviewed')
        plt.ylabel('Number of Users')
        plt.title('User Activity Distribution')
        plt.yscale('log')

        plt.tight_layout()
        plt.savefig(output_path / 'user_activity_distribution.png',
                    dpi=300, bbox_inches='tight')
        plt.close()

    def export_genre_analysis(self, genres: List[str], output_file: str = "genre_analysis.csv"):
        """Export analysis for specific genres"""
        if self.data_source != "database":
            print("❌ Genre analysis requires database format")
            return

        results = []
        for genre in genres:
            analysis = self.analyze_by_genre(genre)
            if analysis:
                results.append({
                    'genre_id': genre,
                    'total_games': analysis['total_games'],
                    'avg_positive_rate': analysis['avg_positive_rate']
                })

        if results:
            pd.DataFrame(results).to_csv(output_file, index=False)
            print(f"✓ Genre analysis exported to {output_file}")

    def close(self):
        """Close database connection"""
        if self.conn:
            self.conn.close()


def main():
    """Main analysis pipeline"""
    print("🎮 Steam Game Market Analysis")
    print("=" * 50)

    # Check available data sources
    db_path = Path("steam_reviews.db")
    csv_path = Path("edges_details_by_review_id.csv")

    if db_path.exists():
        print("📊 Using database format (recommended for large datasets)")
        analyzer = SteamAnalyzer(data_source="database", db_path=str(db_path))
    elif csv_path.exists():
        print("📊 Using CSV format (may be slower for large datasets)")
        analyzer = SteamAnalyzer(data_source="csv")
    else:
        print("❌ No data found. Please run the data assembly process first.")
        return

    try:
        # Basic statistics
        print("\n📈 Basic Dataset Statistics:")
        stats = analyzer.get_basic_stats()
        for key, value in stats.items():
            print(f"   {key.replace('_', ' ').title()}: {value:,}")

        # Top games analysis
        print("\n🏆 Top Games Analysis:")
        top_games = analyzer.analyze_top_games(10)
        print(top_games.to_string(index=False))

        # Create visualizations
        print("\n📊 Creating visualizations...")
        analyzer.create_visualizations()

        # Genre analysis (database only)
        if analyzer.data_source == "database":
            print("\n🎯 Sample Genre Analysis:")
            # Common Steam genre IDs
            common_genres = ["1", "2", "3", "9", "23"]
            analyzer.export_genre_analysis(common_genres)

        print("\n✅ Analysis completed successfully!")
        print("📁 Check the 'analysis_plots' directory for visualizations")

    except Exception as e:
        print(f"❌ Analysis failed: {e}")
    finally:
        analyzer.close()


if __name__ == "__main__":
    main()
