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

uninstall_ch() {
	log "Ch Uninstaller"
	echo

	local os
	os=$(detect_os)

	if [[ ! -d "$CH_HOME" ]]; then
		error "Ch is not installed at $CH_HOME"
	fi

	echo -e "\033[93mRemoving the following:\033[0m"
	echo -e "  - Ch installation directory: $CH_HOME"

	if [[ "$os" == "android" ]]; then
		echo -e "  - Symlink: $PREFIX/bin/ch"
	else
		echo -e "  - Symlink: /usr/local/bin/ch (if exists)"
	fi

	log "Removing Ch installation..."

	if [[ "$os" == "android" ]]; then
		if [[ -L "$PREFIX/bin/ch" ]]; then
			rm -f "$PREFIX/bin/ch"
			log "Removed symlink: $PREFIX/bin/ch"
		fi
	else
		if [[ -L "/usr/local/bin/ch" ]]; then
			if [[ -w "/usr/local/bin" ]]; then
				rm -f "/usr/local/bin/ch"
			else
				if command -v sudo >/dev/null 2>&1; then
					sudo rm -f "/usr/local/bin/ch"
				else
					log "Warning: Could not remove /usr/local/bin/ch (no sudo access)"
				fi
			fi
			log "Removed symlink: /usr/local/bin/ch"
		fi
	fi

	rm -rf "$CH_HOME"
	log "Removed installation directory: $CH_HOME"

	echo
	echo -e "\033[92m✓ Ch has been successfully uninstalled\033[0m"
	exit 0
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

	# Core dependencies (clipboard utilities are optional and auto-detected)
	local deps=("fzf" "curl" "lynx" "yt-dlp")
	local missing_deps=()
	local pip_deps=("ddgr")
	local missing_pip_deps=()

	for dep in "${deps[@]}"; do
		if ! command -v "$dep" >/dev/null 2>&1; then
			missing_deps+=("$dep")
		fi
	done

	# Check for Python dependencies
	for dep in "${pip_deps[@]}"; do
		if ! command -v "$dep" >/dev/null 2>&1; then
			missing_pip_deps+=("$dep")
		fi
	done

	if [[ ${#missing_deps[@]} -eq 0 ]] && [[ ${#missing_pip_deps[@]} -eq 0 ]]; then
		log "All dependencies already installed"
		return
	fi

	if [[ ${#missing_deps[@]} -gt 0 ]]; then
		log "The following system dependencies are missing: ${missing_deps[*]}"
	fi

	if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
		log "The following Python dependencies are missing: ${missing_pip_deps[*]}"
	fi

	case "$os" in
	"macos")
		if ! command -v brew >/dev/null 2>&1; then
			error "Homebrew is required on macOS to install dependencies. Install from https://brew.sh/"
		fi
		for dep in "${missing_deps[@]}"; do
			log "Installing $dep with Homebrew..."
			brew install "$dep"
		done
		# Install Python dependencies
		if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
			if ! command -v pip3 >/dev/null 2>&1 && ! command -v pip >/dev/null 2>&1; then
				log "Installing Python and pip..."
				brew install python
			fi
			for dep in "${missing_pip_deps[@]}"; do
				log "Installing $dep with pip..."
				if command -v pip3 >/dev/null 2>&1; then
					pip3 install "$dep"
				else
					pip install "$dep"
				fi
			done
		fi
		;;
	"android")
		if ! command -v pkg >/dev/null 2>&1; then
			error "pkg package manager not found. Make sure you're running this in Termux."
		fi
		pkg update -y
		for dep in "${missing_deps[@]}"; do
			log "Installing $dep with pkg..."
			pkg install -y "$dep"
		done
		# Install Python dependencies
		if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
			if ! command -v pip >/dev/null 2>&1; then
				log "Installing Python and pip..."
				pkg install -y python
			fi
			for dep in "${missing_pip_deps[@]}"; do
				log "Installing $dep with pip..."
				pip install "$dep"
			done
		fi
		;;
	"linux")
		if command -v sudo >/dev/null 2>&1; then
			if command -v apt-get >/dev/null 2>&1; then
				sudo apt-get update -qq
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with apt-get..."
					sudo apt-get install -y "$dep"
				done
				# Install Python dependencies
				if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
					if ! command -v pip3 >/dev/null 2>&1 && ! command -v pip >/dev/null 2>&1; then
						log "Installing Python and pip..."
						sudo apt-get install -y python3-pip
					fi
					for dep in "${missing_pip_deps[@]}"; do
						log "Installing $dep with pip..."
						if command -v pip3 >/dev/null 2>&1; then
							pip3 install --user "$dep"
						else
							pip install --user "$dep"
						fi
					done
				fi
			elif command -v pacman >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with pacman..."
					sudo pacman -Sy --noconfirm "$dep"
				done
				# Install Python dependencies
				if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
					if ! command -v pip >/dev/null 2>&1; then
						log "Installing Python and pip..."
						sudo pacman -S --noconfirm python-pip
					fi
					for dep in "${missing_pip_deps[@]}"; do
						log "Installing $dep with pip..."
						pip install --user "$dep"
					done
				fi
			elif command -v dnf >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with dnf..."
					sudo dnf install -y "$dep"
				done
				# Install Python dependencies
				if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
					if ! command -v pip3 >/dev/null 2>&1 && ! command -v pip >/dev/null 2>&1; then
						log "Installing Python and pip..."
						sudo dnf install -y python3-pip
					fi
					for dep in "${missing_pip_deps[@]}"; do
						log "Installing $dep with pip..."
						if command -v pip3 >/dev/null 2>&1; then
							pip3 install --user "$dep"
						else
							pip install --user "$dep"
						fi
					done
				fi
			elif command -v yum >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with yum..."
					sudo yum install -y "$dep"
				done
				# Install Python dependencies
				if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
					if ! command -v pip3 >/dev/null 2>&1 && ! command -v pip >/dev/null 2>&1; then
						log "Installing Python and pip..."
						sudo yum install -y python3-pip
					fi
					for dep in "${missing_pip_deps[@]}"; do
						log "Installing $dep with pip..."
						if command -v pip3 >/dev/null 2>&1; then
							pip3 install --user "$dep"
						else
							pip install --user "$dep"
						fi
					done
				fi
			elif command -v zypper >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with zypper..."
					sudo zypper install -y "$dep"
				done
				# Install Python dependencies
				if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
					if ! command -v pip3 >/dev/null 2>&1 && ! command -v pip >/dev/null 2>&1; then
						log "Installing Python and pip..."
						sudo zypper install -y python3-pip
					fi
					for dep in "${missing_pip_deps[@]}"; do
						log "Installing $dep with pip..."
						if command -v pip3 >/dev/null 2>&1; then
							pip3 install --user "$dep"
						else
							pip install --user "$dep"
						fi
					done
				fi
			elif command -v apk >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with apk..."
					sudo apk add "$dep"
				done
				# Install Python dependencies
				if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
					if ! command -v pip3 >/dev/null 2>&1 && ! command -v pip >/dev/null 2>&1; then
						log "Installing Python and pip..."
						sudo apk add py3-pip
					fi
					for dep in "${missing_pip_deps[@]}"; do
						log "Installing $dep with pip..."
						if command -v pip3 >/dev/null 2>&1; then
							pip3 install --user "$dep"
						else
							pip install --user "$dep"
						fi
					done
				fi
			else
				local all_missing=("${missing_deps[@]}")
				if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
					all_missing+=("${missing_pip_deps[@]}")
				fi
				error "Unsupported package manager. Please install manually: ${all_missing[*]}"
			fi
		else
			error "sudo is required to install dependencies on Linux. Please install it first."
		fi

		# Final fallback for Python dependencies if not handled by package manager
		if [[ ${#missing_pip_deps[@]} -gt 0 ]]; then
			log "Attempting to install remaining Python dependencies with pip..."
			if command -v pip3 >/dev/null 2>&1; then
				for dep in "${missing_pip_deps[@]}"; do
					if ! command -v "$dep" >/dev/null 2>&1; then
						log "Installing $dep with pip3..."
						pip3 install --user "$dep" 2>/dev/null || log "Failed to install $dep with pip3"
					fi
				done
			elif command -v pip >/dev/null 2>&1; then
				for dep in "${missing_pip_deps[@]}"; do
					if ! command -v "$dep" >/dev/null 2>&1; then
						log "Installing $dep with pip..."
						pip install --user "$dep" 2>/dev/null || log "Failed to install $dep with pip"
					fi
				done
			fi
		fi
		;;
	esac
}

build_ch() {
	log "Creating installation directory $CH_HOME"
	mkdir -p "$BIN_DIR" || error "Failed to create directory $BIN_DIR"

	log "Creating application temp directory"
	mkdir -p "$CH_HOME/tmp" || error "Failed to create directory $CH_HOME/tmp"

	if [[ -d "$CH_HOME" ]]; then
		log "Removing existing Ch installation..."
		rm -rf "$CH_HOME"
		mkdir -p "$BIN_DIR"
	fi

	log "Building Ch..."
	go mod download || error "Failed to download Go modules"

	local os
	os=$(detect_os)

	if [[ "$os" == "android" ]]; then
		log "Building for Android (disabling CGO)..."
		CGO_ENABLED=0 go build -o "$BIN_DIR/ch" cmd/ch/main.go || error "Failed to build Ch"
	else
		go build -o "$BIN_DIR/ch" cmd/ch/main.go || error "Failed to build Ch"
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
		log "Creating symlink for 'ch' in /usr/local/bin"
		local target_dir="/usr/local/bin"
		local symlink_path="$target_dir/ch"

		if [[ ! -d "$target_dir" ]]; then
			log "Directory $target_dir does not exist. Creating it with sudo."
			if command -v sudo >/dev/null 2>&1; then
				sudo mkdir -p "$target_dir"
			else
				error "sudo is required to create $target_dir. Please create it manually and re-run."
			fi
			if [[ $? -ne 0 ]]; then
				error "Failed to create $target_dir. Please create it manually and re-run the script."
			fi
		fi

		if [[ -w "$target_dir" ]]; then
			ln -sf "$source_path" "$symlink_path"
			log "Symlink created: $symlink_path -> $source_path"
		else
			log "Attempting to create symlink with sudo..."
			if command -v sudo >/dev/null 2>&1; then
				sudo ln -sf "$source_path" "$symlink_path"
			else
				log "sudo not available. Cannot create symlink in $target_dir"
				log "You can still use ch by either:"
				log "  1. Adding $BIN_DIR to your PATH: export PATH=\"$BIN_DIR:\$PATH\""
				log "  2. Creating the symlink manually: sudo ln -sf \"$source_path\" \"$symlink_path\""
				log "  3. Using the full path: $source_path"
				return 0
			fi
			if [[ $? -ne 0 ]]; then
				log "Failed to create symlink in $target_dir"
				log "You can still use ch by either:"
				log "  1. Adding $BIN_DIR to your PATH: export PATH=\"$BIN_DIR:\$PATH\""
				log "  2. Creating the symlink manually: sudo ln -sf \"$source_path\" \"$symlink_path\""
				log "  3. Using the full path: $source_path"
				return 0
			fi
			log "Symlink created with sudo: $symlink_path -> $source_path"
		fi
	fi
}

print_success() {
	local os
	os=$(detect_os)

	echo
	echo -e "\033[92m🎉 Ch installation/update complete!\033[0m"
	echo
	echo -e "Ch is installed in: \033[90m$CH_HOME\033[0m"

	if [[ "$os" == "android" ]]; then
		echo -e "A symlink has been created at \$PREFIX/bin/ch, so you can run 'ch' from anywhere."
		echo
		echo -e "\033[93mImportant:\033[0m"
		echo -e "Make sure '\$PREFIX/bin' is in your \$PATH (it should be by default in Termux)."
		echo -e "You can check by running: \033[90mecho \$PATH\033[0m"
	else
		echo -e "A symlink has been created at /usr/local/bin/ch, so you can run 'ch' from anywhere."
		echo
		echo -e "\033[93mImportant:\033[0m"
		echo -e "Make sure '/usr/local/bin' is in your \$PATH."
		echo -e "You can check by running: \033[90mecho \$PATH\033[0m"
	fi

	echo -e "You may need to restart your terminal session for changes to take effect."
	echo
	echo -e "To get started, simply type:"
	echo -e "  \033[96mch\033[0m"
	echo
	echo -e "If you installed via curl/wget, the cloned repository has been removed."
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

	local os
	os=$(detect_os)

	if [[ "$os" == "android" ]]; then
		log "Building for Android (disabling CGO)..."
		CGO_ENABLED=0 make build || error "Build failed"
	else
		log "Building project..."
		make build || error "Build failed"
	fi

	echo
	echo -e "\033[92m✓ Build complete!\033[0m"
	echo -e "Binary location: \033[90m./bin/ch\033[0m"
}

refresh_deps() {
	log "Refreshing dependencies..."
	go get -u ./... || error "Failed to refresh dependencies"
	go mod tidy || error "Failed to tidy modules"
	log "Dependencies refreshed successfully"
}

show_help() {
	echo "Usage: $0 [OPTIONS]"
	echo ""
	echo "Ch setup script for installation, building, and maintenance"
	echo ""
	echo "Options:"
	echo "  -u, --uninstall     Uninstall Ch from the system"
	echo "  -b, --build         Build Ch locally (requires local repository)"
	echo "  -r, --refresh-deps  Refresh Go dependencies before building (local only)"
	echo "  -h, --help          Show this help message"
	echo ""
	echo "Default behavior: Install Ch (downloads from GitHub if needed)"
	echo ""
	echo "Note: Build options (-b, -r) only work when run locally from the repository,"
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

		trap "log 'Cleaning up temporary files...'; rm -rf '$temp_dir'" EXIT

		log "Cloning Ch repository into a temporary directory..."
		git clone --depth 1 "$REPO_URL" "$temp_dir" || error "Failed to clone the repository."

		cd "$temp_dir" || error "Failed to enter the temporary directory."

		_install_ch_from_repo
	fi
}

main "$@"
