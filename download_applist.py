import requests
import json


def download_steam_applist():
    url = "https://api.steampowered.com/ISteamApps/GetAppList/v2/"

    try:
        response = requests.get(url)

        if response.status_code == 200:
            with open("steam_applist.json", "w", encoding="utf-8") as file:
                json.dump(response.json(), file, indent=2)
            print("Steam app list saved to steam_applist.json")
        else:
            print(f"Request failed with status code: {response.status_code}")

    except requests.exceptions.RequestException as e:
        print(f"Error making request: {e}")


if __name__ == "__main__":
    download_steam_applist()
