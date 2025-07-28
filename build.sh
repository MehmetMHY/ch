#!/bin/bash

show_help() {
	echo "Usage: $0 [OPTIONS]"
	echo ""
	echo "Build script for Go project"
	echo ""
	echo "Options:"
	echo "  -u, --update    Update dependencies before building"
	echo "  -h, --help      Show this help message"
	echo ""
	echo "Default behavior: build the project"
}

update_deps() {
	echo "Updating dependencies..."
	go get -u ./...
	go mod tidy
}

build_project() {
	echo "Building project..."
	go mod download
	make build
}

# Parse arguments
case "${1:-}" in
-h | --help)
	show_help
	exit 0
	;;
-u | --update)
	update_deps
	build_project
	;;
"")
	build_project
	;;
*)
	echo "Unknown option: $1"
	show_help
	exit 1
	;;
esac
