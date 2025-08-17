#!/usr/bin/env bash

show_help() {
	echo "Usage: $0 [OPTIONS] <url|query> [url2] [url3] ..."
	echo ""
	echo "Scrape web content from URLs or search results"
	echo ""
	echo "Options:"
	echo "  -s, --search <query>    Search DuckDuckGo and interactively select URLs to scrape"
	echo "  -h, --help              Show this help message"
	echo ""
	echo "Examples:"
	echo "  $0 https://example.com                      # Scrape a single URL"
	echo "  $0 -s \"what is the goal of life\"            # Search and interactively scrape"
	echo "  $0 --search \"machine learning basics\"       # Search and interactively scrape"
	echo "  $0 https://url1.com https://url2.com       # Scrape multiple URLs"
}

search_and_scrape() {
	local query="$1"

	if ! command -v ddgr &>/dev/null; then
		echo "Error: ddgr is not installed. Please run the install script first."
		exit 1
	fi

	if ! command -v fzf &>/dev/null; then
		echo "Error: fzf is not installed. Please run the install script first."
		exit 1
	fi

	echo "Searching for: $query"
	echo "==============================================="

	# Get search results from ddgr
	local results
	results=$(ddgr --json "$query" 2>/dev/null)

	if [[ -z "$results" ]] || [[ "$results" == "[]" ]]; then
		echo "No search results found for: $query"
		exit 1
	fi

	# Display formatted search results
	echo "$results" | jq -r '.[] | "Title: \(.title)\nURL: \(.url)\nDescription: \(.abstract)\n"'
	echo

	# Ask user if they want to scrape any URLs
	echo "Do you want to scrape any of these URLs? (y/N)"
	read -r response

	if [[ "$response" =~ ^[Yy]$ ]]; then
		echo "Select URLs to scrape (use TAB to select multiple, ENTER to confirm):"
		local selected_urls
		selected_urls=$(echo "$results" | jq -r '.[].url' | fzf --multi --height=40% --reverse --prompt="Select URLs: ")

		if [[ -n "$selected_urls" ]]; then
			echo "Scraping selected URLs..."
			echo "==============================================="
			# Convert selected URLs to array and scrape them
			while IFS= read -r url; do
				scrape_url "$url"
			done <<<"$selected_urls"
		else
			echo "No URLs selected for scraping."
		fi
	else
		echo "Search completed. No scraping performed."
	fi
}

scrape_url() {
	local url="$1"
	echo "=== $url ==="
	if [[ $url =~ (youtube\.com|youtu\.be|m\.youtube\.com|www\.youtube\.com|youtube-nocookie\.com) ]]; then
		echo "--- METADATA ---"
		yt-dlp -j "$url" | jq '{
			title: .title,
			duration: .duration,
			view_count: .view_count,
			like_count: .like_count,
			uploader: .uploader,
			upload_date: .upload_date,
			description: .description,
			tags: .tags,
			thumbnail: .thumbnail,
			channel: .channel,
			author: .uploader
		}'

		echo
		echo "--- SUBTITLES ---"
		BASE=$(mktemp -p "$TMPROOT" ytXXXX)
		yt-dlp --quiet --skip-download \
			--write-auto-subs --sub-lang en --sub-format srt \
			-o "${BASE}.%(ext)s" "$url"

		SRT_FILE=$(echo "${BASE}".*.srt)
		[[ -f $SRT_FILE ]] && cat "$SRT_FILE"
		rm -f "${BASE}".*
	else
		curl -s "$url" | lynx -dump -stdin
	fi
	echo
}

# Parse arguments
case "$1" in
-h | --help)
	show_help
	exit 0
	;;
-s | --search)
	if [[ $# -lt 2 ]]; then
		echo "Error: search option requires a search query"
		echo "Usage: $0 -s \"<search query>\" or $0 --search \"<search query>\""
		exit 1
	fi
	shift
	search_and_scrape "$*"
	exit 0
	;;
"")
	show_help
	exit 1
	;;
esac

TMPROOT="$HOME/.ch/tmp"
mkdir -p "$TMPROOT"

# Regular URL scraping mode
for url in "$@"; do
	scrape_url "$url"
done
