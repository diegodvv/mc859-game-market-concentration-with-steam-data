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

Go to the `graph-analyser` folder and run the `assemble-graph.ipynb` notebook. It contains code to read the data obtained from step 1 and 2 into files of nodes and edges.

Finally, use the `analyse-graph.ipynb` file to compute some statistics of the graph and see some visualizations of the node degree distribution and the concentration of reviews per game with a Lorenz curve plot and the Gini coefficient.


# Data

The data generated from the `assemble-graph.ipynb` is also available on `https://huggingface.co/datasets/diegodvv/steam-games-reviews-and-users-graph/tree/main`