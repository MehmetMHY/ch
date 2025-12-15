#!/usr/bin/env bash

set -euo pipefail

CH_HOME="$HOME/.ch"
BIN_DIR="$CH_HOME/bin"
REPO_URL="https://github.com/MehmetMHY/ch.git"

log() {
	echo -e "\033[96m$1\033[0m"
}

error() {
	echo -e "\033[91mERROR: $1\033[0m" >&2
	exit 1
}

warning() {
	echo -e "\033[93mWarning: $1\033[0m"
}

uninstall_ch() {
	echo -e "\033[91mRemoving Ch installation...\033[0m"

	local os
	os=$(detect_os)

	if [[ ! -d "$CH_HOME" ]]; then
		error "Ch is not installed at $CH_HOME"
	fi

	if [[ "$os" == "android" ]]; then
		if [[ -L "$PREFIX/bin/ch" ]]; then
			rm -f "$PREFIX/bin/ch"
			echo -e "\033[91mRemoved symlink: $PREFIX/bin/ch\033[0m"
		fi
	else
		if [[ -L "/usr/local/bin/ch" ]]; then
			if [[ -w "/usr/local/bin" ]]; then
				rm -f "/usr/local/bin/ch"
			else
				if command -v sudo >/dev/null 2>&1; then
					sudo rm -f "/usr/local/bin/ch"
				else
					warning "Could not remove /usr/local/bin/ch (no sudo access)"
				fi
			fi
			echo -e "\033[91mRemoved symlink: /usr/local/bin/ch\033[0m"
		fi
	fi

	rm -rf "$CH_HOME"
	echo -e "\033[91mRemoved installation directory: $CH_HOME\033[0m"
	echo -e "\033[96mCh has been successfully uninstalled\033[0m"
	exit 0
}

safe_uninstall_ch() {
	log "Ch Safe Uninstaller"
	echo -e "\033[93mThis will remove the following:\033[0m"
	echo -e "\033[93m- Config, history, & sessions: $CH_HOME\033[0m"
	local os
	os=$(detect_os)
	if [[ "$os" == "android" ]]; then
		echo -e "\033[93m- Symlink: $PREFIX/bin/ch\033[0m"
	else
		echo -e "\033[93m- Symlink: /usr/local/bin/ch (if exists)\033[0m"
	fi

	if [[ ! -d "$CH_HOME" ]]; then
		error "Ch is not installed at $CH_HOME"
	fi

	local response
	response=$(safe_input "$(echo -e '\033[92mAre you sure you want to uninstall Ch? \033[91m(y/N) \033[0m')") || response=""
	response=$(echo "$response" | tr '[:upper:]' '[:lower:]')
	if [[ "$response" != "y" ]] && [[ "$response" != "yes" ]]; then
		exit 0
	fi

	uninstall_ch
}

check_go() {
	if ! command -v go >/dev/null 2>&1; then
		error "Go 1.21+ is required but not installed"
	fi

	local go_version
	go_version=$(go version | cut -d' ' -f3 | sed 's/go//')
	if ! go version | grep -qE "go(1\.(2[1-9]|[3-9][0-9])|[2-9]\.)"; then
		error "Go 1.21+ is required (found $go_version)"
	fi
}

detect_os() {
	if [[ "$OSTYPE" == "darwin"* ]]; then
		echo "macos"
	elif [[ "$OSTYPE" == "linux-gnu"* ]] || [[ "$OSTYPE" == "linux-android"* ]]; then
		if [[ -n "${TERMUX_VERSION:-}" ]] || [[ -d "/data/data/com.termux" ]]; then
			echo "android"
		else
			echo "linux"
		fi
	else
		error "Unsupported operating system: $OSTYPE"
	fi
}

install_dependencies() {
	local os
	os=$(detect_os)

	log "Checking system dependencies for $os"

	local required_deps=("fzf")
	local optional_deps=("yt-dlp" "tesseract")

	local missing_required=()
	local missing_optional=()

	for dep in "${required_deps[@]}"; do
		if ! command -v "$dep" >/dev/null 2>&1; then
			missing_required+=("$dep")
		fi
	done

	for dep in "${optional_deps[@]}"; do
		if ! command -v "$dep" >/dev/null 2>&1; then
			missing_optional+=("$dep")
		fi
	done

	if [[ ${#missing_required[@]} -eq 0 ]] && [[ ${#missing_optional[@]} -eq 0 ]]; then
		log "All dependencies already installed"
		return
	fi

	if [[ ${#missing_required[@]} -gt 0 ]]; then
		log "The following required system dependencies are missing: ${missing_required[*]}"
	fi

	if [[ ${#missing_optional[@]} -gt 0 ]]; then
		log "The following optional system dependencies are missing: ${missing_optional[*]}"
	fi

	for dep in "${missing_optional[@]}"; do
		if [[ "$dep" == "tesseract" ]]; then
			warning "Tesseract OCR is not installed. Image-to-text extraction will be disabled."
			warning "The script will attempt to install it. If it fails, you can install it manually (e.g., 'brew install tesseract' or 'sudo apt-get install tesseract-ocr')."
		fi
	done

	case "$os" in
	"macos")
		if ! command -v brew >/dev/null 2>&1; then
			error "Homebrew is required on macOS to install dependencies. Install from https://brew.sh/"
		fi
		for dep in "${missing_required[@]}"; do
			log "Installing required dependency $dep with Homebrew..."
			brew install "$dep" || error "Failed to install required dependency: $dep. Please install it manually."
		done
		for dep in "${missing_optional[@]}"; do
			log "Installing optional dependency $dep with Homebrew..."
			brew install "$dep" || warning "Failed to install optional dependency: $dep."
		done
		;;
	"android")
		if ! command -v pkg >/dev/null 2>&1; then
			error "pkg package manager not found. Make sure you're running this in Termux."
		fi
		pkg update -y
		for dep in "${missing_required[@]}"; do
			log "Installing required dependency $dep with pkg..."
			pkg install -y "$dep" || error "Failed to install required dependency: $dep. Please install it manually."
		done
		for dep in "${missing_optional[@]}"; do
			local install_name="$dep"
			if [[ "$dep" == "tesseract" ]]; then
				install_name="tesseract-ocr"
			fi
			log "Installing optional dependency $install_name with pkg..."
			pkg install -y "$install_name" || warning "Failed to install optional dependency: $dep."
		done
		;;
	"linux")
		if ! command -v sudo >/dev/null 2>&1; then
			error "sudo is required to install dependencies on Linux. Please install it first."
		fi

		local pkg_manager=""
		if command -v apt-get >/dev/null 2>&1; then pkg_manager="apt-get"; fi
		if command -v pacman >/dev/null 2>&1; then pkg_manager="pacman"; fi
		if command -v dnf >/dev/null 2>&1; then pkg_manager="dnf"; fi
		if command -v yum >/dev/null 2>&1; then pkg_manager="yum"; fi
		if command -v zypper >/dev/null 2>&1; then pkg_manager="zypper"; fi
		if command -v apk >/dev/null 2>&1; then pkg_manager="apk"; fi

		if [[ -z "$pkg_manager" ]]; then
			error "Unsupported package manager. Please install manually: fzf (required), ${optional_deps[*]} (optional)"
		fi

		log "Updating package manager..."
		case "$pkg_manager" in
		"apt-get") sudo apt-get update -qq ;;
		"pacman") sudo pacman -Sy --noconfirm ;;
		esac

		for dep in "${missing_required[@]}"; do
			log "Installing required dependency $dep with $pkg_manager..."
			case "$pkg_manager" in
			"apt-get") sudo apt-get install -y "$dep" ;;
			"pacman") sudo pacman -S --noconfirm "$dep" ;;
			"dnf") sudo dnf install -y "$dep" ;;
			"yum") sudo yum install -y "$dep" ;;
			"zypper") sudo zypper install -y "$dep" ;;
			"apk") sudo apk add "$dep" ;;
			esac
			if ! command -v "$dep" >/dev/null 2>&1; then
				error "Failed to install required dependency: $dep. Please install it manually."
			fi
		done

		for dep in "${missing_optional[@]}"; do
			local install_name="$dep"
			if [[ "$dep" == "tesseract" ]]; then
				if [[ "$pkg_manager" == "apt-get" ]]; then
					install_name="tesseract-ocr"
				fi
			fi
			log "Installing optional dependency $install_name with $pkg_manager..."
			case "$pkg_manager" in
			"apt-get") sudo apt-get install -y "$install_name" ;;
			"pacman") sudo pacman -S --noconfirm "$install_name" ;;
			"dnf") sudo dnf install -y "$install_name" ;;
			"yum") sudo yum install -y "$install_name" ;;
			"zypper") sudo zypper install -y "$install_name" ;;
			"apk") sudo apk add "$install_name" ;;
			esac
			if ! command -v "$dep" >/dev/null 2>&1; then
				warning "Failed to install optional dependency: $dep."
			fi
		done
		;;
	esac
}

build_ch() {
	log "Creating installation directory $CH_HOME"
	mkdir -p "$BIN_DIR" || error "Failed to create directory $BIN_DIR"

	log "Creating application temp directory"
	mkdir -p "$CH_HOME/tmp" || error "Failed to create directory $CH_HOME/tmp"

	log "Building Ch..."
	go mod download || error "Failed to download Go modules"

	local bin_path="$BIN_DIR/ch"
	execute_build "direct" "$bin_path"

	chmod +x "$bin_path" || error "Failed to make binary executable"
}

# Helper function to execute build command with OS-specific settings
# Takes two arguments:
#   1. build_method: either "direct" (for go build) or "make" (for make build)
#   2. output_path: path for output binary (only used with direct method)
execute_build() {
	local build_method="$1"
	local output_path="$2"

	local os
	os=$(detect_os)

	if [[ "$os" == "android" ]]; then
		if [[ "$build_method" == "direct" ]]; then
			log "Building for Android (disabling CGO)..."
			CGO_ENABLED=0 go build -o "$output_path" cmd/ch/main.go || error "Failed to build Ch"
		else
			log "Building for Android (disabling CGO)..."
			CGO_ENABLED=0 make build || error "Build failed"
		fi
	elif [[ "$os" == "macos" ]] && [[ "$(uname -m)" == "arm64" ]]; then
		local brew_prefix
		brew_prefix=$(brew --prefix)
		if [[ "$build_method" == "direct" ]]; then
			log "Building for macOS on Apple Silicon with Homebrew flags..."
			CGO_CPPFLAGS="-I${brew_prefix}/include" CGO_LDFLAGS="-L${brew_prefix}/lib" go build -o "$output_path" cmd/ch/main.go || error "Failed to build Ch"
		else
			log "Building for macOS on Apple Silicon with Homebrew flags..."
			export CGO_CPPFLAGS="-I${brew_prefix}/include"
			export CGO_LDFLAGS="-L${brew_prefix}/lib"
			make build || error "Build failed"
		fi
	else
		if [[ "$build_method" == "direct" ]]; then
			go build -o "$output_path" cmd/ch/main.go || error "Failed to build Ch"
		else
			log "Building project..."
			make build || error "Build failed"
		fi
	fi
}

create_symlink() {
	local os
	os=$(detect_os)
	local source_path="$BIN_DIR/ch"

	if [[ "$os" == "android" ]]; then
		log "Creating symlink for 'ch' in \$PREFIX/bin"
		local target_dir="$PREFIX/bin"
		local symlink_path="$target_dir/ch"

		if [[ ! -d "$target_dir" ]]; then
			mkdir -p "$target_dir" || error "Failed to create directory $target_dir"
		fi

		ln -sf "$source_path" "$symlink_path"
		log "Symlink created: $symlink_path -> $source_path"
	else
		log "Attempting to create symlink for 'ch' in a directory in your PATH."
		local target_dir="/usr/local/bin"
		local symlink_path="$target_dir/ch"

		# Try to create symlink without sudo first
		if [[ -d "$target_dir" ]] && [[ -w "$target_dir" ]]; then
			ln -sf "$source_path" "$symlink_path"
			log "Symlink created: $symlink_path -> $source_path"
			return
		fi

		# If it fails, skip symlink creation
		warning "Could not create symlink in $target_dir without elevated permissions."
		SYMLINK_SKIPPED=true
	fi
}

check_api_keys() {
	log "Checking API Key status..."

	local required_keys=("OPENAI_API_KEY")
	local optional_keys=(
		"BRAVE_API_KEY"
		"GROQ_API_KEY"
		"DEEP_SEEK_API_KEY"
		"ANTHROPIC_API_KEY"
		"XAI_API_KEY"
		"TOGETHER_API_KEY"
		"GEMINI_API_KEY"
		"MISTRAL_API_KEY"
	)

	for key in "${required_keys[@]}"; do
		if [[ -v $key ]]; then
			echo -e "\033[92m✓ $key is set\033[0m"
		else
			echo -e "\033[91m✗ $key is not set (Required)\033[0m"
		fi
	done

	for key in "${optional_keys[@]}"; do
		if [[ -v $key ]]; then
			echo -e "\033[92m✓ $key is set\033[0m"
		else
			echo -e "\033[93m- $key is not set (Optional)\033[0m"
		fi
	done

	log "Done checking API key status"
}

print_success() {
	check_api_keys

	local os
	os=$(detect_os)

	echo
	echo -e "\033[92m🎉 Ch installation/update complete!\033[0m"
	echo
	echo -e "Ch is installed in: \033[90m$CH_HOME\033[0m"

	if [[ "$os" == "android" ]]; then
		echo -e "A symlink was created at \$PREFIX/bin/ch"
		echo
		echo -e "\033[93mImportant:\033[0m"
		echo -e "- Make sure '\$PREFIX/bin' is in your \$PATH (should default to Termux)"
		echo -e "- You can check by running: \033[90mecho \$PATH\033[0m"
		echo -e "- You may need to restart your terminal"
		echo -e "- Curl/wget installs should remove cloned repo"
	elif [[ "${SYMLINK_SKIPPED:-false}" == true ]]; then
		echo
		echo -e "\033[93mTo complete the installation, please add Ch to your PATH:\033[0m"
		echo -e "Add the following line to your shell profile (e.g., ~/.zshrc, ~/.bash_profile):"
		echo
		echo -e "\033[92mexport PATH=\"$HOME/.ch/bin:\$PATH\"\033[0m"
		echo
		echo -e "After adding it, restart your shell or run 'source <your_profile_file>'."
	else
		echo -e "A symlink was created at /usr/local/bin/ch"
		echo
		echo -e "\033[93mImportant:\033[0m"
		echo -e "- Make sure '/usr/local/bin' is in your \$PATH"
		echo -e "- You can check by running: \033[90mecho \$PATH\033[0m"
		echo -e "- You may need to restart your terminal"
		echo -e "- Curl/wget installs should remove cloned repo"
	fi

	echo
	echo -e "To get started, simply type:"
	echo -e "\033[91mch\033[0m"
}

check_git_and_pull() {
	if ! command -v git >/dev/null 2>&1; then
		error "Git is required to run the installation script. Please install it first."
	fi
	log "Pulling latest changes from git..."
	git pull || error "Failed to pull latest changes from git"
}

_install_ch_from_repo() {
	log "Starting Ch installation process from local repository..."
	SYMLINK_SKIPPED=false
	mkdir -p "$CH_HOME"
	check_go
	install_dependencies
	build_ch
	create_symlink
	print_success
}

build_only() {
	log "Building Ch (local build only)..."
	check_go

	if [[ ! -f "Makefile" ]]; then
		error "Makefile not found. Please run from the Ch repository root."
	fi

	log "Downloading dependencies..."
	go mod download || error "Failed to download Go modules"

	execute_build "make" ""

	echo
	echo -e "\033[92m✓ Build complete!\033[0m"
	echo -e "Binary location: \033[90m./bin/ch\033[0m"
}

update_cli_tools() {
	log "Updating CLI tools..."

	local os
	os=$(detect_os)

	local system_deps=("fzf" "yt-dlp" "tesseract")

	case "$os" in
	"macos")
		if command -v brew >/dev/null 2>&1; then
			for dep in "${system_deps[@]}"; do
				if command -v "$dep" >/dev/null 2>&1; then
					log "Updating $dep..."
					brew upgrade "$dep" 2>/dev/null || log "$dep already up to date or not installed via brew"
				fi
			done
		fi
		;;
	"android")
		if command -v pkg >/dev/null 2>&1; then
			log "Updating package list..."
			pkg update -y >/dev/null 2>&1
			for dep in "${system_deps[@]}"; do
				if command -v "$dep" >/dev/null 2>&1; then
					log "Updating $dep..."
					pkg upgrade -y "$dep" 2>/dev/null || log "$dep already up to date"
				fi
			done
		fi
		;;
	"linux")
		if command -v sudo >/dev/null 2>&1; then
			local pkg_manager=""
			if command -v apt-get >/dev/null 2>&1; then pkg_manager="apt-get"; fi
			if command -v pacman >/dev/null 2>&1; then pkg_manager="pacman"; fi
			if command -v dnf >/dev/null 2>&1; then pkg_manager="dnf"; fi
			if command -v yum >/dev/null 2>&1; then pkg_manager="yum"; fi
			if command -v zypper >/dev/null 2>&1; then pkg_manager="zypper"; fi
			if command -v apk >/dev/null 2>&1; then pkg_manager="apk"; fi

			if [[ -z "$pkg_manager" ]]; then
				warning "Unsupported package manager. Skipping CLI tool updates."
				return
			fi

			log "Updating package manager..."
			case "$pkg_manager" in
			"apt-get") sudo apt-get update -qq ;;
			"pacman") sudo pacman -Sy --noconfirm ;;
			esac

			for dep in "${system_deps[@]}"; do
				if command -v "$dep" >/dev/null 2>&1; then
					local install_name="$dep"
					if [[ "$dep" == "tesseract" && "$pkg_manager" == "apt-get" ]]; then
						install_name="tesseract-ocr"
					fi
					log "Updating $dep..."
					case "$pkg_manager" in
					"apt-get") sudo apt-get install --only-upgrade -y "$install_name" 2>/dev/null || log "$dep already up to date" ;;
					"pacman") sudo pacman -S --noconfirm "$install_name" 2>/dev/null || log "$dep already up to date" ;;
					"dnf") sudo dnf upgrade -y "$install_name" 2>/dev/null || log "$dep already up to date" ;;
					"yum") sudo yum update -y "$install_name" 2>/dev/null || log "$dep already up to date" ;;
					"zypper") sudo zypper update -y "$install_name" 2>/dev/null || log "$dep already up to date" ;;
					"apk") sudo apk upgrade "$install_name" 2>/dev/null || log "$dep already up to date" ;;
					esac
				fi
			done
		else
			warning "sudo not available - skipping system package updates"
		fi
		;;
	esac

	log "CLI tools update complete"
}

refresh_deps() {
	log "Refreshing dependencies..."
	go get -u ./... || error "Failed to refresh dependencies"
	go mod tidy || error "Failed to tidy modules"
	log "Go dependencies refreshed successfully"

	update_cli_tools
	log "All dependencies refreshed successfully"
}

safe_input() {
	local prompt="$1"
	trap 'echo; return 1' INT
	read -p "$prompt" response || return 1
	trap - INT
	echo "$response"
}

update_version() {
	local makefile="Makefile"

	# Check if Makefile exists
	if [[ ! -f "$makefile" ]]; then
		error "Makefile not found. Please run from the Ch repository root."
	fi

	# Extract current version from Makefile
	local current_version
	current_version=$(grep "^VERSION?=" "$makefile" | cut -d'=' -f2)

	if [[ -z "$current_version" ]]; then
		error "Could not find VERSION in Makefile"
	fi

	echo "Current version: $current_version"

	# Remove 'v' prefix if it exists for parsing
	local version_number="${current_version#v}"

	# Parse version components
	if [[ ! "$version_number" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
		error "Invalid version format. Expected format: v1.2.3 or 1.2.3"
	fi

	local major="${BASH_REMATCH[1]}"
	local minor="${BASH_REMATCH[2]}"
	local patch="${BASH_REMATCH[3]}"

	# Calculate version bumps
	local patch_bump="$major.$minor.$((patch + 1))"
	local minor_bump="$major.$((minor + 1)).0"
	local major_bump="$((major + 1)).0.0"

	echo "Select the new version:"
	echo "1) Patch: v$patch_bump"
	echo "2) Minor: v$minor_bump"
	echo "3) Major: v$major_bump"
	echo "4) Stash: $current_version"
	echo "5) Custom version"

	local choice
	choice=$(safe_input "Enter your choice [1-5]: ") || choice=""

	local new_version
	case "$choice" in
	1)
		new_version="v$patch_bump"
		;;
	2)
		new_version="v$minor_bump"
		;;
	3)
		new_version="v$major_bump"
		;;
	4)
		echo "Keeping current version: $current_version"
		exit 0
		;;
	5)
		local custom
		custom=$(safe_input "Enter custom version (e.g., v1.2.3): ")
		# Ensure it starts with 'v'
		if [[ ! "$custom" =~ ^v ]]; then
			custom="v$custom"
		fi
		new_version="$custom"
		;;
	*)
		error "Invalid choice"
		;;
	esac

	# Update Makefile
	echo "Updating version to: $new_version"

	# Use sed to replace the VERSION line (works on both macOS and Linux)
	if [[ "$OSTYPE" == "darwin"* ]]; then
		sed -i '' "s/^VERSION?=.*/VERSION?=$new_version/" "$makefile"
	else
		sed -i "s/^VERSION?=.*/VERSION?=$new_version/" "$makefile"
	fi

	echo "Version updated to $new_version in Makefile"
	echo "Next steps:"
	echo "1) Commit the Makefile changes"
	echo "2) Build with: make build"
	echo "3) Create release with: make release"
}

show_help() {
	echo "Usage: $0 [OPTIONS]"
	echo ""
	echo "Ch setup script for installation, building, and maintenance"
	echo ""
	echo "Options:"
	echo "  -s, --safe-uninstall     Uninstall Ch with confirmation prompt"
	echo "  -u, --uninstall          Uninstall Ch from the system"
	echo "  -b, --build              Build Ch locally (requires local repository)"
	echo "  -r, --refresh-deps       Refresh Go dependencies before building (local only)"
	echo "  -v, --version            Update version in Makefile (local only)"
	echo "  -h, --help               Show this help message"
	echo ""
	echo "Default behavior: Install Ch (downloads from GitHub if needed)"
	echo ""
	echo "Note: Build options (-b, -r, -v) only work when run locally from the repository,"
	echo "      not through curl/wget installation."
}

main() {
	local build_only_flag=false
	local refresh_deps_flag=false
	local is_remote_install=false

	if [[ ! -t 0 ]] || [[ "${BASH_SOURCE[0]}" == "" ]]; then
		is_remote_install=true
	fi

	while [[ $# -gt 0 ]]; do
		case "$1" in
		-u | --uninstall)
			uninstall_ch
			;;
		-s | --safe-uninstall)
			safe_uninstall_ch
			;;
		-b | --build)
			if [[ "$is_remote_install" == true ]]; then
				error "Build option is only available when running locally from the repository"
			fi
			build_only_flag=true
			;;
		-r | --refresh-deps)
			if [[ "$is_remote_install" == true ]]; then
				error "Refresh deps option is only available when running locally from the repository"
			fi
			refresh_deps_flag=true
			;;
		-v | --version)
			if [[ "$is_remote_install" == true ]]; then
				error "Version option is only available when running locally from the repository"
			fi
			update_version
			exit 0
			;;
		-h | --help)
			show_help
			exit 0
			;;
		*)
			error "Unknown option: $1. Use -h or --help to see available options."
			;;
		esac
		shift
	done

	if [[ "$build_only_flag" == true ]]; then
		if [[ ! -f "go.mod" ]] || [[ ! -f "cmd/ch/main.go" ]]; then
			error "Build option requires running from the Ch repository root directory"
		fi

		if [[ "$refresh_deps_flag" == true ]]; then
			refresh_deps
		fi

		build_only
		exit 0
	fi

	if [ -f "go.mod" ] && [ -f "cmd/ch/main.go" ] && [ -d ".git" ]; then
		log "Running installer from existing local repository."

		if [[ "$refresh_deps_flag" == true ]]; then
			refresh_deps
		fi

		check_git_and_pull
		_install_ch_from_repo
	else
		log "Welcome to the Ch installer!"
		log "This script will download and install Ch on your system."

		if ! command -v git >/dev/null 2>&1; then
			error "Git is required to run this installer. Please install it first."
		fi

		local temp_dir
		temp_dir="$HOME/.ch/tmp/ch-install-$$"
		mkdir -p "$temp_dir" || error "Failed to create temporary directory"

		trap "rm -rf '$temp_dir'" EXIT

		log "Cloning Ch repository into a temporary directory..."
		git clone --depth 1 "$REPO_URL" "$temp_dir" || error "Failed to clone the repository."

		cd "$temp_dir" || error "Failed to enter the temporary directory."

		_install_ch_from_repo

		log "Cleaning up temporary files..."
	fi
}

main "$@"
