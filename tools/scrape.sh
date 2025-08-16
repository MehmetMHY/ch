#!/usr/bin/env bash

deps=("yt-dlp" "curl" "lynx" "jq")
missing=()
for dep in "${deps[@]}"; do
	command -v "$dep" >/dev/null 2>&1 || missing+=("$dep")
done
if [[ ${#missing[@]} -ne 0 ]]; then
	if [[ -n "${TERMUX_VERSION:-}" ]] || [[ -d "/data/data/com.termux" ]]; then
		pkg install -y "${missing[@]}"
	elif command -v apt-get >/dev/null 2>&1; then
		sudo apt-get install -y "${missing[@]}"
	elif command -v pacman >/dev/null 2>&1; then
		sudo pacman -Sy --noconfirm "${missing[@]}"
	elif command -v dnf >/dev/null 2>&1; then
		sudo dnf install -y "${missing[@]}"
	elif command -v yum >/dev/null 2>&1; then
		sudo yum install -y "${missing[@]}"
	elif command -v zypper >/dev/null 2>&1; then
		sudo zypper install -y "${missing[@]}"
	elif command -v apk >/dev/null 2>&1; then
		sudo apk add "${missing[@]}"
	elif command -v brew >/dev/null 2>&1; then
		brew install "${missing[@]}"
	else
		echo "Missing dependencies: ${missing[*]}"
		exit 1
	fi
fi

[[ $# -ge 1 ]] || {
	echo "usage: $0 <url> [url2] [url3] ..."
	exit 1
}

TMPROOT="$HOME/.ch/tmp"
mkdir -p "$TMPROOT"

for url in "$@"; do
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
done
