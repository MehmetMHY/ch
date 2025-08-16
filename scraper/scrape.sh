#!/usr/bin/env bash

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
