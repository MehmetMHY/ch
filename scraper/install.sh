#!/usr/bin/env bash

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
