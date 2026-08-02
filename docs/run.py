#!/usr/bin/env python3

from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import webbrowser
import threading
import functools
import signal
import sys

HOST = "127.0.0.1"
START_PORT = 8000
MAX_PORT = 8099


class ReusableHTTPServer(ThreadingHTTPServer):
    daemon_threads = True


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
    site_dir = Path(__file__).resolve().parent
    handler = functools.partial(SimpleHTTPRequestHandler, directory=str(site_dir))

    server, port = make_server(handler)
    if server is None:
        return 1

    url = f"http://localhost:{port}"

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
