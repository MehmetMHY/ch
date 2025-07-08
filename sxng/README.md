# Cha Uses SearXNG Search Engine

## About

SearXNG is an open-source search engine. The catch with it is that you have to host it. This is easy to do with Docker and the scripts located in this directory but it's not as convenient as using DuckDuckGo's free API. But, DuckDuckGo's free API is limited and you can run into rate limit issues with enough use. Also, using search API(s) can be difficult to setup and you have to manage another API key. Due to this, setting up SearXNG is not required to use Cha but it is heavily recommended you utilize SearXNG. For more information, visit the [SearXNG documentation](https://docs.searxng.org/).

## How To Setup

1. Make sure to install and setup [Docker](https://www.docker.com/)

2. Install Python dependencies (optional, for automatic JSON format configuration):
   ```bash
   pip3 install -r requirements.txt
   ```
3. Run the setup script and follow each instruction:
   ```bash
   python3 run.py
   ```

**Note**: The `run.py` script will automatically enable JSON format in your `settings.yml` file if PyYAML is installed and JSON format is not already enabled. This is required for the `!w` web search feature in Cha to work properly. If PyYAML is not installed, the script will still work but you'll need to manually add JSON format to your settings.yml.

## How To Query the SearXNG API

After starting your SearXNG instance (by default at `http://localhost:8080`), you can make search queries directly to the API. Use an HTTP `GET` request to the `/search` endpoint with the following parameters:

- `q`: Your search query (required)
- `format`: Response format, should be `"json"` (required)
- `time_range`: Filter results by time (optional, values: `"day"`, `"month"`, `"year"`)
- Additional filters or parameters as supported by SearXNG

#### Example Request (using `curl`):

```bash
curl -G "http://localhost:8080/search" \
     --data-urlencode "q=your search query" \
     --data-urlencode "format=json"
```

#### With a Time Filter:

```bash
curl -G "http://localhost:8080/search" \
     --data-urlencode "q=your search query" \
     --data-urlencode "format=json" \
     --data-urlencode "time_range=month"
```

#### Example Python (using `requests`):

```python
import requests

base_url = "http://localhost:8080"
params = {"q": "your search query", "format": "json", "time_range": "month"}
headers = {"User-Agent": "Mozilla/5.0", "Accept": "application/json"}

response = requests.get(f"{base_url}/search", params=params, headers=headers)
data = response.json()
print(data)
```

The API will return a JSON object containing your search results.
