# Weather Service

A high-performance weather query application that provides average temperature data by aggregating requests and using multiple weather API providers.

## Installation and Setup

### Prerequisites

- Docker and Docker Compose
- Go 1.18 or higher (for local development)
- WeatherStack API key (free tier is sufficient)
- WeatherAPI API key (free tier is sufficient)

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
 ## Running the test script with the more rigorous test cases
```bash
1 - chmod +x test_weather_service.sh

2 - ./test_weather_service.sh
````


### Database Results

After running the tests, the database should contain entries for each location queried:

| id | location  | service_1_temperature | service_2_temperature | request_count | created_at                  |
|----|-----------|----------------------|----------------------|---------------|----------------------------|
| 12 | Istanbul  | 14.00                | 14.00                | 1             | 2025-03-11 17:55:11.680... |
| 13 | London    | 7.10                 | 7.00                 | 5             | 2025-03-11 17:55:16.899... |
| 14 | Paris     | 8.20                 | 8.00                 | 10            | 2025-03-11 17:55:17.171... |
| 15 | New York  | 0.00                 | 17.00                | 1             | 2025-03-11 17:55:22.547... |
| 16 | Tokyo     | 11.00                | 11.00                | 1             | 2025-03-11 17:55:23.021... |
| 17 | Berlin    | 4.20                 | 4.00                 | 1             | 2025-03-11 17:55:23.440... |
| 18 | Sydney    | 19.20                | 19.00                | 1             | 2025-03-11 17:55:23.943... |
| 19 | Moscow    | 8.10                 | 8.00                 | 1             | 2025-03-11 17:55:24.461... |
| 20 | Rome      | 14.00                | 14.00                | 2             | 2025-03-11 17:55:29.708... |

Note how the `request_count` column shows the number of requests that were aggregated for each location.

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
