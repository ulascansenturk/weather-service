# Weather Service

A high-performance weather query application that provides average temperature data by aggregating requests and using multiple weather API providers.

## Installation and Setup

### Prerequisites

- Docker and Docker Compose
- Go 1.18 or higher (for local development)

### Environment Setup

1. Clone the repository:
```bash
git clone https://github.com/ulascansenturk/weather-service.git
cd weather-service
```

2. Create the environment file:
```bash
make env
```

3. Start the application:
```bash
make run-app
```

The service will be available at http://localhost:3000

## Running Tests

### Run integration tests:
```bash
make run-integration-test
```

### Run all tests:
```bash
make run-all-tests
```

## API Usage

### Get Temperature

```
GET /weather?q={location}
```

Where `{location}` is the name of a city or location.

#### Example Request

```bash
curl --location 'localhost:3000/weather?q=istanbul'
```

#### Example Responses

Successful response (both APIs):
```json
{
  "location": "istanbul",
  "temperature": 17.1
}
```

Partial success response (one API failed):
```json
{
  "location": "new york",
  "temperature": 3,
  "warning": "Using only second weather API data"
}
```

## Cleanup

To stop the service and clean up all resources:

```bash
make clean
```
