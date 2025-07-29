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
	elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
		echo "linux"
	else
		error "Unsupported operating system: $OSTYPE"
	fi
}

install_dependencies() {
	local os
	os=$(detect_os)

	log "Checking system dependencies for $os"

	local deps=("fzf")
	local missing_deps=()

	for dep in "${deps[@]}"; do
		if ! command -v "$dep" >/dev/null 2>&1; then
			missing_deps+=("$dep")
		fi
	done

	if [[ ${#missing_deps[@]} -eq 0 ]]; then
		log "All dependencies already installed"
		return
	fi

	log "The following dependencies are missing: ${missing_deps[*]}"
	echo -n -e "\033[93mDo you want to install them? (y/N): \033[0m"
	read -r response
	if [[ ! "$response" =~ ^[Yy]$ ]]; then
		error "Installation aborted. Please install dependencies manually."
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
		;;
	"linux")
		if command -v sudo >/dev/null 2>&1; then
			if command -v apt-get >/dev/null 2>&1; then
				sudo apt-get update -qq
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with apt-get..."
					sudo apt-get install -y "$dep"
				done
			elif command -v pacman >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with pacman..."
					sudo pacman -Sy --noconfirm "$dep"
				done
			elif command -v dnf >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with dnf..."
					sudo dnf install -y "$dep"
				done
			elif command -v yum >/dev/null 2>&1; then
				for dep in "${missing_deps[@]}"; do
					log "Installing $dep with yum..."
					sudo yum install -y "$dep"
				done
			else
				error "Unsupported package manager. Please install manually: ${missing_deps[*]}"
			fi
		else
			error "sudo is required to install dependencies on Linux. Please install it first."
		fi
		;;
	esac
}

build_ch() {
	log "Creating installation directory $CH_HOME"
	mkdir -p "$BIN_DIR" || error "Failed to create directory $BIN_DIR"

	if [[ -d "$CH_HOME" ]]; then
		echo -e "\033[93mAn existing Ch installation was found.\033[0m"
		echo -n -e "\033[91mDo you want to remove it and reinstall/update? (y/N): \033[0m"
		read -r response
		if [[ "$response" =~ ^[Yy]$ ]]; then
			log "Removing existing installation..."
			rm -rf "$CH_HOME"
			mkdir -p "$BIN_DIR"
		else
			log "Update aborted. Keeping existing installation."
			exit 0
		fi
	fi

	log "Building Ch..."
	go mod download || error "Failed to download Go modules"
	go build -o "$BIN_DIR/ch" cmd/ch/main.go || error "Failed to build Ch"
}

create_symlink() {
	log "Creating symlink for 'ch' in /usr/local/bin"
	local target_dir="/usr/local/bin"
	local symlink_path="$target_dir/ch"
	local source_path="$BIN_DIR/ch"

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
			error "sudo is required to create symlink in $target_dir. Please create it manually."
		fi
		if [[ $? -ne 0 ]]; then
			error "Failed to create symlink. Please try creating it manually: sudo ln -sf \"$source_path\" \"$symlink_path\""
		fi
		log "Symlink created with sudo: $symlink_path -> $source_path"
	fi
}

print_success() {
	echo
	echo -e "\033[92m🎉 Ch installation/update complete!\033[0m"
	echo
	echo -e "Ch is installed in: \033[90m$CH_HOME\033[0m"
	echo -e "A symlink has been created at /usr/local/bin/ch, so you can run 'ch' from anywhere."
	echo
	echo -e "\033[93mImportant:\033[0m"
	echo -e "Make sure '/usr/local/bin' is in your \$PATH."
	echo -e "You can check by running: \033[90mecho \$PATH\033[0m"
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

main() {
	if [ -f "go.mod" ] && [ -f "cmd/ch/main.go" ] && [ -d ".git" ]; then
		log "Running installer from existing local repository."
		check_git_and_pull
		_install_ch_from_repo
	else
		log "Welcome to the Ch installer!"
		log "This script will download and install Ch on your system."

		if ! command -v git >/dev/null 2>&1; then
			error "Git is required to run this installer. Please install it first."
		fi

		local temp_dir
		temp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t 'ch-install')
		trap "log 'Cleaning up temporary files...'; rm -rf '$temp_dir'" EXIT

		log "Cloning Ch repository into a temporary directory..."
		git clone --depth 1 "$REPO_URL" "$temp_dir" || error "Failed to clone the repository."

		cd "$temp_dir" || error "Failed to enter the temporary directory."

		_install_ch_from_repo
	fi
}

main "$@"
