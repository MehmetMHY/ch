#!/bin/bash

# display help message
show_help() {
	cat <<EOF
Usage: build.sh [OPTION]

A utility tool for managing documentation and images.

OPTIONS:
  -h, --help      Show this help message
  -r, --run       Open the documentation in the default browser
  -c, --convert   Convert and update image files using ImageMagick

You can install ImageMagick from here: https://imagemagick.org/
EOF
}

# run the documentation (start http server)
run_docs() {
	# check if python is available
	if ! command -v python3 &>/dev/null && ! command -v python &>/dev/null; then
		echo "Error: Python is required but not installed."
		exit 1
	fi

	# determine which python command to use
	local python_cmd
	if command -v python3 &>/dev/null; then
		python_cmd="python3"
	else
		python_cmd="python"
	fi

	echo "Starting HTTP server on http://localhost:8000"
	echo "Press Ctrl+C to stop the server"

	# open the browser
	if [[ "$OSTYPE" == "darwin"* ]]; then
		open "http://localhost:8000" &
	elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
		xdg-open "http://localhost:8000" &
	elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]]; then
		start "http://localhost:8000" &
	else
		echo "unsupported os"
		exit 1
	fi

	# start the http server
	$python_cmd -m http.server 8000
}

# convert images with imagemagick
convert_images() {
	# check if imagemagick is installed
	if ! command -v convert &>/dev/null; then
		echo "Error: ImageMagick is required but not installed."
		echo "Install it with: brew install imagemagick (macOS) or apt-get install imagemagick (Linux)"
		exit 1
	fi

	# check if the source logo file exists
	if [ ! -f "logo.png" ]; then
		echo "Error: logo.png not found in current directory!"
		exit 1
	fi

	# setup base variables
	LOGO_FILE="logo.png"
	LOGO_SIZE=$(identify -format "%wx%h" "$LOGO_FILE" 2>/dev/null || echo "unknown")

	echo "Loaded $LOGO_FILE (size: $LOGO_SIZE)"

	# clear metadata from logo.png before processing
	echo "Clearing metadata from $LOGO_FILE..."
	if mogrify -strip "$LOGO_FILE" 2>/dev/null; then
		echo "  Metadata cleared successfully"
		# get new size after metadata removal
		LOGO_SIZE=$(identify -format "%wx%h" "$LOGO_FILE" 2>/dev/null || echo "unknown")
		echo "  Size after metadata removal: $LOGO_SIZE"
	else
		echo "  Warning: Failed to clear metadata from $LOGO_FILE (mogrify may not be installed)"
		echo "  Continuing without metadata removal..."
	fi

	# list of supported image file extensions
	IMAGE_EXTENSIONS="png ico jpg jpeg gif bmp svg webp tiff tif"

	# list of files to skip during processing
	IGNORE_FILES="color_logo.png thumbnail.png"

	# counter for tracking updates
	UPDATED_COUNT=0

	echo "Updating images..."

	# generate a thumbnail image with the logo centered on a dominant color background
	generate_thumbnail() {
		local logo_file="$1"
		local thumbnail_file="thumbnail.png"

		echo "Generating $thumbnail_file..."

		# get thumbnail resolution (check existing file or use default 1024x1024)
		local thumb_size
		local thumb_height
		if [ -f "$thumbnail_file" ]; then
			thumb_size=$(identify -format "%wx%h" "$thumbnail_file" 2>/dev/null)
			echo "  Existing thumbnail size: $thumb_size"
		else
			thumb_size="1024x1024"
			echo "  Using default thumbnail size: $thumb_size"
		fi

		# extract the height value from the size string (e.g., "1024x1024" -> "1024")
		thumb_height=$(echo "$thumb_size" | cut -d'x' -f2)

		# calculate logo height as 1/3 of thumbnail height to ensure it fits properly
		local logo_height=$((thumb_height / 3))
		echo "  Resizing logo to height: ${logo_height}px"

		# find the most popular color in the logo using histogram analysis
		local dominant_color=$(convert "$logo_file" \
			-alpha off \
			-colors 256 \
			-depth 8 \
			-format "%c" histogram:info:- 2>/dev/null |
			sort -rn |
			head -1 |
			grep -oE "#[0-9A-Fa-f]{6}" |
			head -1)

		# if that didn't work, try a simpler approach
		if [ -z "$dominant_color" ]; then
			echo "  Trying alternative color detection method..."
			dominant_color=$(convert "$logo_file" \
				-scale 1x1! \
				-alpha off \
				-format "%[pixel:p{0,0}]" info:- 2>/dev/null)
		fi

		# fallback to white if color detection fails
		if [ -z "$dominant_color" ]; then
			echo "  Could not determine most popular color, using white"
			dominant_color="white"
		else
			echo "  Most popular color found: $dominant_color"
		fi

		# create a temporary directory for intermediate files
		local temp_dir=$(mktemp -d)
		local resized_logo="$temp_dir/resized_logo.png"

		# resize logo to 1/3 height while maintaining aspect ratio (x = maintain ratio, just set height)
		if ! convert "$logo_file" -resize "x${logo_height}" "$resized_logo"; then
			echo "  Failed to resize logo"
			rm -rf "$temp_dir"
			return 1
		fi

		# create final thumbnail: colored background + resized logo centered on top
		if convert \
			-size "$thumb_size" \
			"xc:$dominant_color" \
			"$resized_logo" \
			-gravity center \
			-composite \
			"$thumbnail_file"; then
			echo "  Successfully generated $thumbnail_file"
			rm -rf "$temp_dir"
			return 0
		else
			echo "  Failed to generate $thumbnail_file"
			rm -rf "$temp_dir"
			return 1
		fi
	}

	# update an ico file by extracting its sizes and creating versions at those sizes from the logo
	update_ico_file() {
		local ico_file="$1"
		local logo_file="$2"

		echo "Processing ICO file: $ico_file"

		# create a temporary directory for intermediate png files
		local temp_dir=$(mktemp -d)

		# extract all image sizes contained in the ico file
		local ico_sizes=$(identify -format "%wx%h " "$ico_file" 2>/dev/null)

		# if we can't read the ico file, use standard favicon sizes as fallback
		if [ -z "$ico_sizes" ]; then
			echo "  Could not read existing ICO file, using standard favicon sizes"
			ico_sizes="16x16 32x32 48x48 64x64"
		fi

		echo "  ICO contains sizes: $ico_sizes"

		# create resized versions of the logo for each size in the ico
		local success=true
		for size in $ico_sizes; do
			# remove trailing space and parse width x height using regex
			size=$(echo "$size" | sed 's/ $//')
			if [[ "$size" =~ ([0-9]+)x([0-9]+) ]]; then
				local width=${BASH_REMATCH[1]}
				local height=${BASH_REMATCH[2]}

				local temp_png="$temp_dir/${width}x${height}.png"

				# resize logo to exact size (! forces exact dimensions without maintaining aspect ratio)
				if convert "$logo_file" -resize "${width}x${height}!" "$temp_png"; then
					echo "    Created ${width}x${height} version"
				else
					echo "    Failed to create ${width}x${height} version"
					success=false
				fi
			fi
		done

		# combine all resized pngs back into a single ico file
		if [ "$success" = true ] && ls "$temp_dir"/*.png >/dev/null 2>&1; then
			if convert "$temp_dir"/*.png "$ico_file"; then
				echo "  Successfully updated ICO file with multiple sizes"
				rm -rf "$temp_dir"
				return 0
			else
				echo "  Failed to create new ICO file"
				rm -rf "$temp_dir"
				return 1
			fi
		else
			echo "  No valid PNGs created for ICO, skipping"
			rm -rf "$temp_dir"
			return 1
		fi
	}

	# generate thumbnail first
	generate_thumbnail "$LOGO_FILE"

	# main loop: process all image files in the current directory
	for file in *; do
		# skip directories and only process regular files
		if [ ! -f "$file" ]; then
			continue
		fi

		# check if file is in the ignore list (skip processing if it is)
		IGNORE_THIS=false
		for ignore in $IGNORE_FILES; do
			if [ "$file" = "$ignore" ]; then
				IGNORE_THIS=true
				break
			fi
		done

		if [ "$IGNORE_THIS" = true ]; then
			continue
		fi

		# extract and normalize file extension to lowercase for comparison
		EXT="${file##*.}"
		EXT_LOWER=$(echo "$EXT" | tr '[:upper:]' '[:lower:]')

		# check if this file type is in our supported list
		SUPPORTED=false
		for ext in $IMAGE_EXTENSIONS; do
			if [ "$EXT_LOWER" = "$ext" ]; then
				SUPPORTED=true
				break
			fi
		done

		# skip files with unsupported extensions
		if [ "$SUPPORTED" = false ]; then
			continue
		fi

		# don't process the source logo file itself
		if [ "$file" = "$LOGO_FILE" ]; then
			continue
		fi

		# ico files require special multi-size handling
		if [ "$EXT_LOWER" = "ico" ]; then
			if update_ico_file "$file" "$LOGO_FILE"; then
				echo "Updated $file"
				((UPDATED_COUNT++))
			else
				echo "Failed to update $file"
			fi
		else
			# for regular image files, get current dimensions and overwrite with resized logo
			TARGET_SIZE=$(identify -format "%wx%h" "$file" 2>/dev/null || echo "unknown")
			TARGET_FORMAT=$(identify -format "%m" "$file" 2>/dev/null || echo "UNKNOWN")

			# resize logo to match target dimensions (! forces exact size ignoring aspect ratio)
			if convert "$LOGO_FILE" -resize "${TARGET_SIZE}!" "$file"; then
				echo "Updated $file (${TARGET_SIZE})"
				((UPDATED_COUNT++))
			else
				echo "Error updating $file"
			fi
		fi
	done

	# final summary
	echo ""
	echo "Successfully updated $UPDATED_COUNT image files!"
}

# main logic
case "${1:-}" in
-h | --help)
	show_help
	;;
-c | --convert)
	convert_images
	;;
-r | --run | "")
	# default behavior: run the documentation
	run_docs
	;;
*)
	echo "Error: Unknown option '$1'"
	echo "Use 'build.sh -h' for help"
	exit 1
	;;
esac
