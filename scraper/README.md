# Scraper

A simple web content extraction tool that complements Ch by providing focused scraping capabilities without bloating the main CLI.

## Features

- YouTube video metadata and subtitle extraction
- General web page content extraction using lynx
- Clean, structured output perfect for AI analysis
- Minimal dependencies with cross-platform support

## Installation

```bash
cd scraper/
./install.sh
```

This installs dependencies (yt-dlp, curl, lynx, jq) and places the `scrape` command in your system PATH.

## Usage

```bash
# scrape web pages
scrape https://example.com

# extract YouTube video data and subtitles
scrape https://youtube.com/watch?v=VIDEO_ID

# process multiple URLs
scrape https://site1.com https://site2.com
```

## Integration with Ch

After installation, use with Ch through shell session recording:

```bash
ch
!x scrape https://example.com
```

The scraped content becomes available as context for your AI conversation.
