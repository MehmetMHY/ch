#!/usr/bin/env bash

set -euo pipefail

CH_HOME="$HOME/.ch"
BIN_DIR="$CH_HOME/bin"
VENV_DIR="$CH_HOME/venv"
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
					warning "Could not remove /usr/local/bin/ch (no sudo access)"
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

	local required_deps=("fzf")
	local optional_deps=("yt-dlp")
	pip_deps=()

	local missing_required=()
	local missing_optional=()
	local missing_pip=()

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

	for dep in "${pip_deps[@]}"; do
		if ! command -v "$dep" >/dev/null 2>&1 && ! [[ -f "$VENV_DIR/bin/$dep" ]]; then
			missing_pip+=("$dep")
		fi
	done

	if [[ ${#missing_required[@]} -eq 0 ]] && [[ ${#missing_optional[@]} -eq 0 ]] && [[ ${#missing_pip[@]} -eq 0 ]]; then
		log "All dependencies already installed"
		return
	fi

	if [[ ${#missing_required[@]} -gt 0 ]]; then
		log "The following required system dependencies are missing: ${missing_required[*]}"
	fi
	if [[ ${#missing_optional[@]} -gt 0 ]]; then
		log "The following optional system dependencies are missing: ${missing_optional[*]}"
	fi
	if [[ ${#missing_pip[@]} -gt 0 ]]; then
		log "The following optional Python dependencies are missing: ${missing_pip[*]}"
	fi

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
		if [[ ${#missing_pip[@]} -gt 0 ]]; then
			if ! command -v pip3 >/dev/null 2>&1 && ! command -v pip >/dev/null 2>&1; then
				log "Installing Python and pip..."
				brew install python || warning "Failed to install Python. Pip dependencies will be skipped."
			fi
			for dep in "${missing_pip[@]}"; do
				log "Installing $dep with pip..."
				if command -v pip3 >/dev/null 2>&1; then
					pip3 install "$dep" || warning "Failed to install optional dependency: $dep."
				else
					pip install "$dep" || warning "Failed to install optional dependency: $dep."
				fi
			done
		fi
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
			log "Installing optional dependency $dep with pkg..."
			pkg install -y "$dep" || warning "Failed to install optional dependency: $dep."
		done
		if [[ ${#missing_pip[@]} -gt 0 ]]; then
			if ! command -v pip >/dev/null 2>&1; then
				log "Installing Python and pip..."
				pkg install -y python || warning "Failed to install Python. Pip dependencies will be skipped."
			fi
			for dep in "${missing_pip[@]}"; do
				log "Installing $dep with pip..."
				pip install "$dep" || warning "Failed to install optional dependency: $dep."
			done
		fi
		;;
	"linux")
		if ! command -v sudo >/dev/null 2>&1; then
			error "sudo is required to install dependencies on Linux. Please install it first."
		fi

		local all_deps=("${missing_required[@]}" "${missing_optional[@]}")
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
			log "Installing optional dependency $dep with $pkg_manager..."
			case "$pkg_manager" in
			"apt-get") sudo apt-get install -y "$dep" ;;
			"pacman") sudo pacman -S --noconfirm "$dep" ;;
			"dnf") sudo dnf install -y "$dep" ;;
			"yum") sudo yum install -y "$dep" ;;
			"zypper") sudo zypper install -y "$dep" ;;
			"apk") sudo apk add "$dep" ;;
			esac
			if ! command -v "$dep" >/dev/null 2>&1; then
				warning "Failed to install optional dependency: $dep."
			fi
		done

		if [[ ${#missing_pip[@]} -gt 0 ]]; then
			log "Setting up Python virtual environment for Ch dependencies..."
			if ! command -v python3 >/dev/null 2>&1; then
				warning "python3 is not installed. Skipping Python dependencies."
			else
				if ! python3 -c "import venv" >/dev/null 2>&1; then
					log "Python 'venv' module not found, attempting to install..."
					case "$pkg_manager" in
					"apt-get") sudo apt-get install -y python3-venv ;;
					"dnf" | "yum") sudo "$pkg_manager" install -y python3-virtualenv ;;
					*)
						warning "Could not automatically install 'venv' module. Please install python3-venv or equivalent."
						;;
					esac
				fi

				if python3 -c "import venv" >/dev/null 2>&1; then
					python3 -m venv "$VENV_DIR" || warning "Failed to create Python virtual environment."
					for dep in "${missing_pip[@]}"; do
						log "Installing $dep with pip into virtual environment..."
						"$VENV_DIR/bin/pip" install "$dep" || warning "Failed to install optional dependency: $dep."
					done
				else
					warning "Python 'venv' module is required but not available. Skipping Python dependencies."
				fi
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

	log "Building Ch..."
	go mod download || error "Failed to download Go modules"

	local os
	os=$(detect_os)
	local bin_path="$BIN_DIR/ch-bin"

	if [[ "$os" == "android" ]]; then
		log "Building for Android (disabling CGO)..."
		CGO_ENABLED=0 go build -o "$bin_path" cmd/ch/main.go || error "Failed to build Ch"
	else
		go build -o "$bin_path" cmd/ch/main.go || error "Failed to build Ch"
	fi

	log "Creating wrapper script..."
	local wrapper_path="$BIN_DIR/ch"
	cat >"$wrapper_path" <<EOF
#!/usr/bin/env bash
CH_HOME="\$HOME/.ch"
VENV_DIR="\$CH_HOME/venv"
if [[ -d "\$VENV_DIR" ]]; then
    export PATH="\$VENV_DIR/bin:\$PATH"
fi
exec "\$CH_HOME/bin/ch-bin" "\$@"
EOF

	chmod +x "$wrapper_path" || error "Failed to make wrapper script executable"
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
				warning "sudo not available. Cannot create symlink in $target_dir"
				log "You can still use ch by either:"
				log "  1. Adding $BIN_DIR to your PATH: export PATH=\"$BIN_DIR:\$PATH\""
				log "  2. Creating the symlink manually: sudo ln -sf \"$source_path\" \"$symlink_path\""
				log "  3. Using the full path: $source_path"
				return 0
			fi
			if [[ $? -ne 0 ]]; then
				warning "Failed to create symlink in $target_dir"
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
		if [[ -n "${!key-}" ]]; then
			echo -e "\033[92m✓ $key is set\033[0m"
		else
			echo -e "\033[91m✗ $key is not set (Required)\033[0m"
		fi
	done

	for key in "${optional_keys[@]}"; do
		if [[ -n "${!key-}" ]]; then
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
		echo -e "A symlink has been created at \$PREFIX/bin/ch, so you can run 'ch' from anywhere."
		echo
		echo -e "\033[93mImportant:\033[0m"
		echo -e "- Make sure '\$PREFIX/bin' is in your \$PATH (it should be by default in Termux)."
		echo -e "- You can check by running: \033[90mecho \$PATH\033[0m"
		echo -e "- You may need to restart your terminal."
	else
		echo -e "A symlink has been created at /usr/local/bin/ch, so you can run 'ch' from anywhere."
		echo
		echo -e "\033[93mImportant:\033[0m"
		echo -e "- Make sure '/usr/local/bin' is in your \$PATH."
		echo -e "- You can check by running: \033[90mecho \$PATH\033[0m"
		echo -e "- You may need to restart your terminal."
	fi

	echo
	echo -e "To get started, simply type:"
	echo -e "  \033[96mch\033[0m"
	echo
	echo -e "If you installed via curl/wget, the cloned repository should have been removed."
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

update_cli_tools() {
	log "Updating CLI tools..."

	local os
	os=$(detect_os)

	local system_deps=("fzf")
	local pip_deps=("yt-dlp")

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
			if command -v fzf >/dev/null 2>&1 && [[ -d ~/.fzf ]]; then
				log "Updating fzf from git..."
				(cd ~/.fzf && git pull && ./install --all >/dev/null 2>&1) || warning "Failed to update fzf from git"
			else
				if command -v apt-get >/dev/null 2>&1; then
					log "Updating package list..."
					sudo apt-get update -qq >/dev/null 2>&1
					for dep in "${system_deps[@]}"; do
						if command -v "$dep" >/dev/null 2>&1; then
							log "Updating $dep..."
							sudo apt-get install --only-upgrade -y "$dep" 2>/dev/null || log "$dep already up to date"
						fi
					done
				elif command -v pacman >/dev/null 2>&1; then
					log "Updating package database..."
					sudo pacman -Sy >/dev/null 2>&1
					for dep in "${system_deps[@]}"; do
						if command -v "$dep" >/dev/null 2>&1; then
							log "Updating $dep..."
							sudo pacman -S --noconfirm "$dep" 2>/dev/null || log "$dep already up to date"
						fi
					done
				elif command -v dnf >/dev/null 2>&1; then
					for dep in "${system_deps[@]}"; do
						if command -v "$dep" >/dev/null 2>&1; then
							log "Updating $dep..."
							sudo dnf upgrade -y "$dep" 2>/dev/null || log "$dep already up to date"
						fi
					done
				elif command -v yum >/dev/null 2>&1; then
					for dep in "${system_deps[@]}"; do
						if command -v "$dep" >/dev/null 2>&1; then
							log "Updating $dep..."
							sudo yum update -y "$dep" 2>/dev/null || log "$dep already up to date"
						fi
					done
				elif command -v zypper >/dev/null 2>&1; then
					for dep in "${system_deps[@]}"; do
						if command -v "$dep" >/dev/null 2>&1; then
							log "Updating $dep..."
							sudo zypper update -y "$dep" 2>/dev/null || log "$dep already up to date"
						fi
					done
				elif command -v apk >/dev/null 2>&1; then
					log "Updating package index..."
					sudo apk update >/dev/null 2>&1
					for dep in "${system_deps[@]}"; do
						if command -v "$dep" >/dev/null 2>&1; then
							log "Updating $dep..."
							sudo apk upgrade "$dep" 2>/dev/null || log "$dep already up to date"
						fi
					done
				else
					log "No supported package manager found for updating system packages"
				fi
			fi
		else
			warning "sudo not available - skipping system package updates"
		fi
		;;
	esac

	local venv_pip="$VENV_DIR/bin/pip"
	if [[ -f "$venv_pip" ]]; then
		log "Updating Python dependencies in virtual environment..."
		for dep in "${pip_deps[@]}"; do
			if "$venv_pip" list | grep -qi "^$dep "; then
				log "Updating $dep..."
				"$venv_pip" install --upgrade "$dep" 2>/dev/null || warning "Failed to update $dep"
			fi
		done
	else
		for dep in "${pip_deps[@]}"; do
			if command -v "$dep" >/dev/null 2>&1; then
				log "Updating $dep..."
				case "$dep" in
				"yt-dlp")
					if [[ "$os" != "android" ]]; then
						yt-dlp --update 2>/dev/null || {
							if command -v pip3 >/dev/null 2>&1; then
								pip3 install --upgrade --user yt-dlp 2>/dev/null || warning "Failed to update yt-dlp"
							else
								pip install --upgrade --user yt-dlp 2>/dev/null || warning "Failed to update yt-dlp"
							fi
						}
					else
						if command -v pip >/dev/null 2>&1; then
							pip install --upgrade yt-dlp 2>/dev/null || warning "Failed to update yt-dlp"
						fi
					fi
					;;
				esac
			fi
		done
	fi

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
		temp_dir="$HOME/.ch/tmp/ch-install-$"
		mkdir -p "$temp_dir" || error "Failed to create temporary directory"

		trap "log 'Cleaning up temporary files...'; rm -rf '$temp_dir'" EXIT

		log "Cloning Ch repository into a temporary directory..."
		git clone --depth 1 "$REPO_URL" "$temp_dir" || error "Failed to clone the repository."

		cd "$temp_dir" || error "Failed to enter the temporary directory."

		_install_ch_from_repo
	fi
}

main "$@"
