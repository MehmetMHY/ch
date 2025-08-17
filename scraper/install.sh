#!/usr/bin/env bash

show_help() {
	echo "Usage: $0 [OPTIONS]"
	echo ""
	echo "Scraper installation script"
	echo ""
	echo "Options:"
	echo "  -u, --uninstall     Uninstall scraper from the system"
	echo "  -h, --help          Show this help message"
	echo ""
	echo "Default behavior: Install scraper and its dependencies"
}

uninstall_scraper() {
	echo "Scraper Uninstaller"
	echo

	if [[ -f "$INSTALL_DIR/scrape" ]]; then
		echo "Removing scraper from $INSTALL_DIR/scrape"
		if [[ "$INSTALL_DIR" == "/usr/local/bin" ]]; then
			if [[ -w "$INSTALL_DIR" ]]; then
				rm -f "$INSTALL_DIR/scrape"
			else
				if command -v sudo &>/dev/null; then
					sudo rm -f "$INSTALL_DIR/scrape"
				else
					echo "Warning: Could not remove $INSTALL_DIR/scrape (no sudo access)"
					exit 1
				fi
			fi
		else
			rm -f "$INSTALL_DIR/scrape"
		fi
		echo "✓ Scraper has been successfully uninstalled"
	else
		echo "Scraper is not installed at $INSTALL_DIR/scrape"
		exit 1
	fi
	exit 0
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
	case "$1" in
	-u | --uninstall)
		# Set install directory for uninstall
		if [[ -n "$PREFIX" ]] && command -v pkg &>/dev/null; then
			INSTALL_DIR="$PREFIX/bin"
		else
			INSTALL_DIR="/usr/local/bin"
		fi
		uninstall_scraper
		;;
	-h | --help)
		show_help
		exit 0
		;;
	*)
		echo "Unknown option: $1. Use -h or --help to see available options."
		exit 1
		;;
	esac
	shift
done

# Set install directory based on environment
if [[ -n "$PREFIX" ]] && command -v pkg &>/dev/null; then
	INSTALL_DIR="$PREFIX/bin" # Termux uses $PREFIX/bin
else
	INSTALL_DIR="/usr/local/bin"
fi

if [[ -n "$PREFIX" ]] && command -v pkg &>/dev/null; then
	echo "Detected Termux pkg..."
	if ! command -v yt-dlp &>/dev/null || ! command -v curl &>/dev/null || ! command -v lynx &>/dev/null || ! command -v jq &>/dev/null; then
		echo "Updating package list..."
		pkg update
	fi
	if ! command -v yt-dlp &>/dev/null; then
		echo "yt-dlp not found, installing via pkg..."
		pkg install -y yt-dlp
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via pkg..."
		pkg install -y curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via pkg..."
		pkg install -y lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via pkg..."
		pkg install -y jq
	fi
elif command -v brew &>/dev/null; then
	echo "Detected Homebrew (brew)..."
	if ! command -v yt-dlp &>/dev/null; then
		echo "yt-dlp not found, installing via Homebrew..."
		brew install yt-dlp
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via Homebrew..."
		brew install curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via Homebrew..."
		brew install lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via Homebrew..."
		brew install jq
	fi
elif command -v apt &>/dev/null; then
	echo "Detected APT..."
	if ! command -v yt-dlp &>/dev/null || ! command -v curl &>/dev/null || ! command -v lynx &>/dev/null || ! command -v jq &>/dev/null; then
		echo "Updating package list..."
		sudo apt update
	fi
	if ! command -v yt-dlp &>/dev/null; then
		echo "yt-dlp not found, installing via APT..."
		sudo apt install -y yt-dlp
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via APT..."
		sudo apt install -y curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via APT..."
		sudo apt install -y lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via APT..."
		sudo apt install -y jq
	fi
elif command -v pacman &>/dev/null; then
	echo "Detected Pacman..."
	if ! command -v yt-dlp &>/dev/null || ! command -v curl &>/dev/null || ! command -v lynx &>/dev/null || ! command -v jq &>/dev/null; then
		echo "Synchronizing package databases..."
		sudo pacman -Sy --noconfirm
	fi
	if ! command -v yt-dlp &>/dev/null; then
		echo "yt-dlp not found, installing via Pacman..."
		sudo pacman -S --noconfirm yt-dlp
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via Pacman..."
		sudo pacman -S --noconfirm curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via Pacman..."
		sudo pacman -S --noconfirm lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via Pacman..."
		sudo pacman -S --noconfirm jq
	fi
elif command -v yum &>/dev/null; then
	echo "Detected YUM..."
	if ! command -v yt-dlp &>/dev/null || ! command -v curl &>/dev/null || ! command -v lynx &>/dev/null || ! command -v jq &>/dev/null; then
		echo "Updating package list..."
		sudo yum update -y
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via YUM..."
		sudo yum install -y curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via YUM..."
		sudo yum install -y lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via YUM..."
		sudo yum install -y jq
	fi
	if ! command -v pip &>/dev/null; then
		echo "pip not found, installing python3-pip via YUM..."
		sudo yum install -y python3-pip
	fi
elif command -v dnf &>/dev/null; then
	echo "Detected DNF..."
	if ! command -v yt-dlp &>/dev/null || ! command -v curl &>/dev/null || ! command -v lynx &>/dev/null || ! command -v jq &>/dev/null; then
		echo "Updating package list..."
		sudo dnf update -y
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via DNF..."
		sudo dnf install -y curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via DNF..."
		sudo dnf install -y lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via DNF..."
		sudo dnf install -y jq
	fi
	if ! command -v pip &>/dev/null; then
		echo "pip not found, installing python3-pip via DNF..."
		sudo dnf install -y python3-pip
	fi
elif command -v zypper &>/dev/null; then
	echo "Detected Zypper..."
	if ! command -v yt-dlp &>/dev/null || ! command -v curl &>/dev/null || ! command -v lynx &>/dev/null || ! command -v jq &>/dev/null; then
		echo "Refreshing repositories..."
		sudo zypper refresh
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via Zypper..."
		sudo zypper install -y curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via Zypper..."
		sudo zypper install -y lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via Zypper..."
		sudo zypper install -y jq
	fi
	if ! command -v pip &>/dev/null; then
		echo "pip not found, installing python3-pip via Zypper..."
		sudo zypper install -y python3-pip
	fi
elif command -v emerge &>/dev/null; then
	echo "Detected Portage..."
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via Portage..."
		sudo emerge --ask=n net-misc/curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via Portage..."
		sudo emerge --ask=n www-client/lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via Portage..."
		sudo emerge --ask=n app-misc/jq
	fi
	if ! command -v pip &>/dev/null; then
		echo "pip not found, installing pip via Portage..."
		sudo emerge --ask=n dev-python/pip
	fi
elif command -v xbps-install &>/dev/null; then
	echo "Detected XBPS..."
	if ! command -v yt-dlp &>/dev/null || ! command -v curl &>/dev/null || ! command -v lynx &>/dev/null || ! command -v jq &>/dev/null; then
		echo "Synchronizing repositories..."
		sudo xbps-install -S
	fi
	if ! command -v yt-dlp &>/dev/null; then
		echo "yt-dlp not found, installing via XBPS..."
		sudo xbps-install -y yt-dlp
	fi
	if ! command -v curl &>/dev/null; then
		echo "curl not found, installing via XBPS..."
		sudo xbps-install -y curl
	fi
	if ! command -v lynx &>/dev/null; then
		echo "lynx not found, installing via XBPS..."
		sudo xbps-install -y lynx
	fi
	if ! command -v jq &>/dev/null; then
		echo "jq not found, installing via XBPS..."
		sudo xbps-install -y jq
	fi
	if ! command -v pip &>/dev/null; then
		echo "pip not found, installing python3-pip via XBPS..."
		sudo xbps-install -y python3-pip
	fi
else
	echo "Warning: No supported package manager (brew, apt, pkg, pacman, yum, dnf, zypper, emerge, xbps-install) detected."
	echo "Please ensure all dependencies are installed manually."
fi

if ! command -v yt-dlp &>/dev/null; then
	echo "yt-dlp not found, installing via pip..."
	pip install --break-system-packages yt-dlp
fi

if [[ ! -d "$INSTALL_DIR" ]]; then
	echo "Directory $INSTALL_DIR does not exist - cannot continue."
	exit 1
fi

cp scrape.sh "$INSTALL_DIR/scrape"
chmod +x "$INSTALL_DIR/scrape"
echo "Installation completed successfully!"
