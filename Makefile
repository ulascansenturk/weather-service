env:
	cp -v .env.example .env

clean:
	docker-compose down --remove-orphans --volumes

run-app:
	docker-compose up
run-integration-test:
	cd internal/api/v1/handlers/integrationtests && go test -run TestWeatherService -v
run-all-tests:
	go test -v ./...

