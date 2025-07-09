# SearXNG Search Engine for Cha

SearXNG is an open-source search engine you host yourself. It's recommended for Cha to enable the `!w` web search feature, avoiding API limits and key management found in other search APIs.

## Setup

1. Install [Docker](https://www.docker.com/).

2. (Optional) Install Python dependencies:

   ```bash
   pip3 install -r requirements.txt
   ```

3. Run the setup script:

   ```bash
   python3 run.py
   ```

   This configures JSON response format in `settings.yml` automatically if PyYAML is installed. Otherwise, add JSON format manually.

## Running

Start SearXNG (usually at `http://localhost:8080`) to enable Cha's web search.

## Querying SearXNG API

Use HTTP GET `/search` with parameters:

- `q`: search query (required)
- `format`: must be `"json"` (required)
- `time_range`: optional filter (`"day"`, `"month"`, `"year"`)

Example curl request:

```bash
curl -G "http://localhost:8080/search" --data-urlencode "q=your query" --data-urlencode "format=json"
```

Example Python:

```python
import requests

params = {"q": "your query", "format": "json"}
response = requests.get("http://localhost:8080/search", params=params, headers={"User-Agent": "Mozilla/5.0"})
print(response.json())
```
