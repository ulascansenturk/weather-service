#!/bin/sh
set -e

sleep 5

# Run migrations
echo "Running database migrations..."
/app/db/scripts/migrate.sh

# Start the main application
echo "Starting the application..."
exec go run ./cmd/server
