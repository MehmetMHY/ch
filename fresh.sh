#!/usr/bin/env bash
# Self-contained test of the real `curl | bash` installer from the README.
#
# Builds a minimal fresh-machine image (Ubuntu + Go only, everything else left
# for install.sh to install itself), runs the real install command from the
# README inside a throwaway container, and reports pass/fail. The Dockerfile is
# embedded below so this is the only file you need.
#
#   ./fresh.sh
set -uo pipefail

IMAGE_NAME="ch-install-test"
GO_VERSION="1.26.5"
INSTALL_URL="https://raw.githubusercontent.com/MehmetMHY/ch/main/install.sh"

# Minimal fresh-machine Dockerfile, streamed to `docker build -` via stdin.
# Only Go 1.26.5+, git, and curl are pre-installed (what install.sh assumes
# already exists). fzf, yt-dlp, tesseract, the ch binary, and the
# /usr/local/bin symlink are all installed by install.sh at runtime, exactly
# as they would be for a real user. No build context is needed since the
# Dockerfile copies nothing from the host.
echo "==> Building fresh-machine test image..."
if ! docker build -t "$IMAGE_NAME" - <<DOCKERFILE; then
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \\
    PATH=/usr/local/go/bin:\${PATH}

RUN apt-get update && apt-get install -y --no-install-recommends \\
        ca-certificates \\
        curl \\
        git \\
    && rm -rf /var/lib/apt/lists/*

RUN set -eux; \\
    arch="\$(dpkg --print-architecture)"; \\
    case "\$arch" in \\
        amd64) goarch=amd64 ;; \\
        arm64) goarch=arm64 ;; \\
        *) echo "unsupported architecture: \$arch" >&2; exit 1 ;; \\
    esac; \\
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-\${goarch}.tar.gz" -o /tmp/go.tar.gz; \\
    tar -C /usr/local -xzf /tmp/go.tar.gz; \\
    rm /tmp/go.tar.gz; \\
    go version

WORKDIR /root
CMD ["bash"]
DOCKERFILE
	echo
	echo "FAIL: docker build failed"
	exit 1
fi
echo "OK: image built"
echo

echo "==> Running the real installer inside a fresh container..."
echo "----------------------------------------------------------"
output=$(docker run --rm "$IMAGE_NAME" bash -c "
	set -eo pipefail
	curl -fsSL '$INSTALL_URL' | bash
	echo '---VERIFY---'
	command -v ch
	ch -h | head -5
" 2>&1)
status=$?
echo "$output"
echo "----------------------------------------------------------"
echo

echo "================ RESULT ================"
if [[ $status -eq 0 ]] && echo "$output" | grep -q "installation/update complete" && echo "$output" | grep -q "/usr/local/bin/ch"; then
	echo "PASS: install.sh completed and ch is runnable from a clean container"
else
	echo "FAIL: install or verification step failed (exit code $status)"
fi
echo "=========================================="

exit $status
