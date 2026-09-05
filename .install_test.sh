#!/usr/bin/env bash

# This script is a self-contained integration test for Ch's curl-based install method. It builds a minimal Ubuntu Docker image with a basic setup, installs Ch using curl, and then verifies that Ch was installed correctly. Installer output is streamed while being retained for the final result checks.

set -uo pipefail

IMAGE_NAME="ch-install-test"
GO_VERSION="1.26.5"
INSTALL_URL="https://raw.githubusercontent.com/MehmetMHY/ch/main/install.sh"

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
echo "    This can take several minutes while dependencies are installed and Ch is built."
echo "----------------------------------------------------------"
OUTPUT_FILE=$(mktemp "${TMPDIR:-/tmp}/ch-install-test.XXXXXX")
trap 'rm -f "$OUTPUT_FILE"' EXIT

docker run --rm "$IMAGE_NAME" bash -c "
	set -eo pipefail
	curl -fsSL '$INSTALL_URL' | bash
	echo '---VERIFY---'
	command -v ch
	ch -h | head -5
" 2>&1 | tee "$OUTPUT_FILE"
pipe_status=("${PIPESTATUS[@]}")
status=${pipe_status[0]}
if [[ $status -eq 0 ]] && [[ ${pipe_status[1]} -ne 0 ]]; then
	status=${pipe_status[1]}
fi
output=$(<"$OUTPUT_FILE")
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
