#!/usr/bin/env python3

from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import webbrowser
import threading
import functools
import argparse
import signal
import sys
import os
import io
import re

HOST = "127.0.0.1"
START_PORT = 8000
MAX_PORT = 8099

CANONICAL_TAG = re.compile(rb"<link[^>]*rel=[\"']canonical[\"'][^>]*>", re.I)
TAG_HREF = re.compile(rb"href=[\"']([^\"']+)[\"']", re.I)


class ReusableHTTPServer(ThreadingHTTPServer):
    daemon_threads = True


def site_origin(body):
    tag = CANONICAL_TAG.search(body)
    if not tag:
        return None

    href = TAG_HREF.search(tag.group(0))
    if not href:
        return None

    origin = href.group(1).decode("utf-8", "replace").strip().rstrip("/")
    if not origin.startswith(("http://", "https://")):
        return None

    return origin.encode()


def localize(body, local_origin):
    origin = site_origin(body)
    if not origin or origin == local_origin or origin not in body:
        return body

    return body.replace(origin, local_origin)


class LocalPreviewHandler(SimpleHTTPRequestHandler):
    local_origin = b""

    def end_headers(self):
        self.send_header("Cache-Control", "no-store")
        super().end_headers()

    def send_head(self):
        path = self.translate_path(self.path)

        if os.path.isdir(path):
            index = os.path.join(path, "index.html")
            if not self.path.endswith("/") or not os.path.isfile(index):
                return super().send_head()
            path = index
        elif not path.endswith((".html", ".htm")):
            return super().send_head()

        try:
            original = Path(path).read_bytes()
        except OSError:
            return super().send_head()

        body = localize(original, self.local_origin)
        if body is original:
            return super().send_head()

        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()

        return io.BytesIO(body)


def make_server(handler):
    last_error = None
    for port in range(START_PORT, MAX_PORT + 1):
        try:
            return ReusableHTTPServer((HOST, port), handler), port
        except OSError as exc:
            last_error = exc

    print(
        f"Error: could not start HTTP server on ports {START_PORT}-{MAX_PORT}: {last_error}",
        file=sys.stderr,
    )

    return None, None


def main():
    parser = argparse.ArgumentParser(
        prog="run.py",
        description="Easily run a local dev server for simple websites",
    )
    parser.add_argument(
        "-n",
        "--no-open",
        action="store_true",
        help="do not open the browser automatically on start",
    )
    args = parser.parse_args()

    site_dir = Path(__file__).resolve().parent
    handler = functools.partial(LocalPreviewHandler, directory=str(site_dir))

    server, port = make_server(handler)
    if server is None:
        return 1

    url = f"http://localhost:{port}"
    LocalPreviewHandler.local_origin = url.encode()

    server_thread = threading.Thread(
        target=server.serve_forever, name="docs-http-server"
    )
    server_thread.start()

    def stop(_signum, _frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, stop)
    if hasattr(signal, "SIGHUP"):
        signal.signal(signal.SIGHUP, stop)

    print(f"Starting HTTP server on {url}")
    print("Press Ctrl+C or Ctrl+D to stop the server")
    print(f"Serving {site_dir} at {url}")
    if not args.no_open:
        webbrowser.open(url)

    try:
        while True:
            if sys.stdin.readline() == "":
                break
    except KeyboardInterrupt:
        pass
    finally:
        server.shutdown()
        server.server_close()
        server_thread.join()

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
