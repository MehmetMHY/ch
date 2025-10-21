# Ch Documentation

## About

The official website for [ch](../README.md), a lightweight AI CLI tool for terminal-based AI interactions. The site is hosted on **GitHub Pages** at https://mehmetmhy.github.io/ch/

## Contents

- **index.html**: Where the entire website is stored and loaded from, built using vanilla HTML/CSS/JavaScript.

- **build.sh**: Utility script for managing/developing the website.

## Quick Start

### Get Script's Help Page

```bash
./build.sh --help
```

### Run Website Locally

```bash
./build.sh
# or
./build.sh --run
```

This opens the website locally in your browser.

### Update Website's Images

```bash
# update/change the "logo.png" image then run this command
./build.sh --convert
```

This converts and updates all image files using [ImageMagick](https://imagemagick.org/) based on the `logo.png` image.

**Requirements**

For this feature to work, install [ImageMagick](https://imagemagick.org/):

```bash
# macOS
brew install imagemagick

# Ubuntu/Debian
sudo apt-get install imagemagick

# Fedora/RHEL
sudo dnf install ImageMagick

# Arch
sudo pacman -S imagemagick

# Alpine
apk add imagemagick

# Windows (Chocolatey)
choco install imagemagick

# or visit https://imagemagick.org/
```

## License

Licensed under the MIT License. See [LICENSE](../LICENSE) for details.
