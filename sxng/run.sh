# default values
DEFAULT_PORT=8080
DEFAULT_NAME="searxng-search"
DEFAULT_URL="http://localhost"

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
	echo "Error: Docker is not running. Please start Docker and try again."
	exit 1
fi

# set instance's port number
read -p "Enter port (default $DEFAULT_PORT): " PORT
PORT=${PORT:-$DEFAULT_PORT}

# set instance's name
read -p "Enter instance name (default $DEFAULT_NAME): " NAME
NAME=${NAME:-$DEFAULT_NAME}

# set instance's base URL
read -p "Enter base URL (default $DEFAULT_URL): " BASE_URL
BASE_URL=${BASE_URL:-$DEFAULT_URL}

# check if an instance is already running on the specified port
RUNNING_CONTAINER=$(docker ps --filter "publish=$PORT" --format "{{.ID}}" 2>/dev/null)
if [ ! -z "$RUNNING_CONTAINER" ]; then
	read -p "An instance is already running on port $PORT. Stop it? (y/n): " STOP
	if [[ $STOP == "y" ]]; then
		docker stop $RUNNING_CONTAINER
	else
		echo "Exiting without changes."
		exit 0
	fi
fi

# ask about updating the image
read -p "Do you want to update the searxng image? (y/n): " UPDATE
if [[ $UPDATE == "y" ]]; then
	echo "Pulling latest searxng image..."
	docker pull searxng/searxng
fi

# check and modify settings.yml to enable JSON format if needed
if [ -f "searxng/settings.yml" ]; then
	echo "Checking SearXNG settings for JSON format support..."
	
	# Check if json format is already enabled (exclude comments)
	if grep -A 10 "formats:" searxng/settings.yml | grep -E "^[[:space:]]*- json" >/dev/null; then
		echo "JSON format already enabled in settings.yml"
	else
		echo "Enabling JSON format in settings.yml..."
		# Use sed to add json format after the html format line
		sed -i.bak '/formats:/,/^[[:space:]]*- html/ {
			/^[[:space:]]*- html/ a\
    - json
		}' searxng/settings.yml
		
		if [ $? -eq 0 ]; then
			echo "Successfully enabled JSON format in settings.yml"
		else
			echo "Warning: Could not automatically enable JSON format. You may need to manually add 'json' to the formats section in settings.yml"
		fi
	fi
else
	echo "Note: settings.yml not found. SearXNG will use default settings."
	echo "JSON format may not be available until you configure settings.yml"
fi

# run the container
echo "Starting SearXNG container..."
docker run --rm \
	-d -p ${PORT}:8080 \
	-v "${PWD}/searxng:/etc/searxng" \
	-e "BASE_URL=${BASE_URL}:${PORT}/" \
	-e "INSTANCE_NAME=${NAME}" \
	--name $NAME \
	searxng/searxng

if [ $? -eq 0 ]; then
	echo "SearXNG instance '$NAME' is running on ${BASE_URL}:${PORT}"
else
	echo "Error: Failed to start SearXNG container. Please check Docker logs for more details."
	exit 1
fi
