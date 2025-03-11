echo "Running migration on: ${DATABASE_HOST}:${DATABASE_PORT}"
migrate -path ./db/migrations \
  -database "postgresql://${DATABASE_USER}:${DATABASE_PASSWORD}@${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_NAME}?sslmode=disable"\
  up
