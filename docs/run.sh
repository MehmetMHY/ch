#!/bin/bash

# check if python is available
if ! command -v python3 &>/dev/null && ! command -v python &>/dev/null; then
	echo "Error: Python is required but not installed."
	exit 1
fi

# determine which python command to use
if command -v python3 &>/dev/null; then
	python_cmd="python3"
else
	python_cmd="python"
fi

# simple prints
echo "Starting HTTP server on http://localhost:8000"
echo "Press Ctrl+C to stop the server"

# open the browser
if [[ "$OSTYPE" == "darwin"* ]]; then
	open "http://localhost:8000" &
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
	xdg-open "http://localhost:8000" &
elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]]; then
	start "http://localhost:8000" &
else
	echo "unsupported os"
	exit 1
fi

# start the http server
$python_cmd -m http.server 8000
