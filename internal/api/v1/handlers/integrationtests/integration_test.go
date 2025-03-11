package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"ulascansenturk/weather-service/internal/api/v1/handlers"
	"ulascansenturk/weather-service/internal/db/weatherquery"
	"ulascansenturk/weather-service/internal/inmemorycache"
	"ulascansenturk/weather-service/internal/mocks"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pgTestContainers "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ulascansenturk/weather-service/internal/service"
)

var (
	postgresContainer *pgTestContainers.PostgresContainer
	sharedDB          *gorm.DB
)

type testSetup struct {
	handler       *handlers.WeatherHandler
	weatherAPI    *mocks.MockWeatherAPIService
	repository    weatherquery.Repository
	cacheProvider *mocks.MockCache
	aggregator    service.WeatherRequestAggregator
	db            *gorm.DB
}

const (
	dbName     = "test_api_database"
	dbUser     = "test_user"
	dbPassword = "test_password"
)

func init() {
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

func SetupPostgres(t *testing.T) (*gorm.DB, func()) {
	if sharedDB != nil {
		err := sharedDB.Migrator().DropTable(&weatherquery.WeatherQuery{})
		require.NoError(t, err)

		err = sharedDB.AutoMigrate(&weatherquery.WeatherQuery{})
		require.NoError(t, err)

		return sharedDB, func() {}
	}

	log.Info().Msg("Setting up new PostgreSQL container")

	ctx := context.Background()

	var err error
	postgresContainer, err = pgTestContainers.Run(ctx,
		"postgres:13.3",
		pgTestContainers.WithDatabase(dbName),
		pgTestContainers.WithUsername(dbUser),
		pgTestContainers.WithPassword(dbPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second)),
	)
	require.NoError(t, err)

	host, err := postgresContainer.Host(context.Background())
	require.NoError(t, err)

	endpoint, err := postgresContainer.Endpoint(context.Background(), "")
	require.NoError(t, err)

	parts := strings.Split(endpoint, ":")
	port := parts[1]

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, dbUser, dbPassword, dbName,
	)

	sharedDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	log.Info().Msgf("Connected to database: %s on %s:%s", dbName, host, port)

	sqlDB, err := sharedDB.DB()
	require.NoError(t, err)

	err = sqlDB.Ping()
	require.NoError(t, err)

	err = sharedDB.AutoMigrate(&weatherquery.WeatherQuery{})
	require.NoError(t, err)

	return sharedDB, func() {
		if postgresContainer != nil {
			log.Info().Msg("Terminating PostgreSQL container")
			if err := postgresContainer.Terminate(context.Background()); err != nil {
				log.Error().Err(err).Msg("Failed to terminate PostgreSQL container")
			}
		}
	}
}

func setupTest(t *testing.T, maxWaitTime time.Duration) *testSetup {
	weatherAPIMock := mocks.NewMockWeatherAPIService(t)
	cacheProviderMock := mocks.NewMockCache(t)

	db, _ := SetupPostgres(t)

	repository := weatherquery.NewRepository(db)

	aggregator := service.NewWeatherRequestAggregator(
		weatherAPIMock,
		cacheProviderMock,
		repository,
		10,
		maxWaitTime,
		30*time.Minute,
		5*time.Minute,
	)

	weatherService := service.NewWeatherService(aggregator)

	handler := handlers.NewWeatherHandler(weatherService, 10*time.Second)

	return &testSetup{
		handler:       handler,
		weatherAPI:    weatherAPIMock,
		repository:    repository,
		cacheProvider: cacheProviderMock,
		aggregator:    aggregator,
		db:            db,
	}
}

func TestWeatherService(t *testing.T) {
	db, cleanup := SetupPostgres(t)
	defer cleanup()

	t.Run("SingleRequest_5SecondWait", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: SingleRequest_5SecondWait")

		ts := setupTest(t, 5*time.Second)
		defer ts.aggregator.Shutdown()

		ts.cacheProvider.On("Get", "Istanbul").Return(nil, false, nil)
		ts.weatherAPI.On("GetWeatherData", "Istanbul").Return(25.5, 24.5, true, true, nil)
		ts.cacheProvider.On("Set", "Istanbul", mock.Anything, 30*time.Minute).Return(nil)

		startTime := time.Now()

		req := httptest.NewRequest("GET", "/weather?q=Istanbul", nil)
		w := httptest.NewRecorder()

		ts.handler.GetWeather(w, req)

		elapsedTime := time.Since(startTime)
		if elapsedTime < 4*time.Second {
			t.Errorf("Request completed too quickly (%v), should be held for ~5 seconds", elapsedTime)
			log.Error().Dur("elapsed", elapsedTime).Msg("❌ TEST FAILED: Request not held for expected time")
		} else {
			log.Info().Dur("elapsed", elapsedTime).Msg("✅ Request held for expected time")
		}

		assert.Equal(t, http.StatusOK, w.Code)

		var response handlers.WeatherResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Istanbul", response.Location)
		assert.Equal(t, 25.0, response.Temperature)
		assert.Empty(t, response.Warning)

		time.Sleep(100 * time.Millisecond)

		var query weatherquery.WeatherQuery
		result := db.Where("location = ?", "Istanbul").Order("created_at DESC").First(&query)
		if result.Error != nil {
			log.Error().Err(result.Error).Msg("❌ TEST FAILED: Database query failed")
			t.Fail()
		}

		assert.Equal(t, "Istanbul", query.Location)
		assert.Equal(t, 25.5, query.Service1Temperature)
		assert.Equal(t, 24.5, query.Service2Temperature)
		assert.Equal(t, 1, query.RequestCount)

		log.Info().Msg("✅ TEST PASSED: SingleRequest_5SecondWait")
	})

	t.Run("JoiningExistingQueue", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: JoiningExistingQueue")

		ts := setupTest(t, 5*time.Second)
		defer ts.aggregator.Shutdown()

		location := "London"

		ts.cacheProvider.On("Get", location).Return(nil, false, nil).Times(2)
		ts.weatherAPI.On("GetWeatherData", location).Return(22.5, 21.5, true, true, nil).Once()
		ts.cacheProvider.On("Set", location, mock.Anything, 30*time.Minute).Return(nil).Once()

		var firstResponseTime time.Duration
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()

			startTime := time.Now()

			req := httptest.NewRequest("GET", "/weather?q="+location, nil)
			w := httptest.NewRecorder()

			ts.handler.GetWeather(w, req)

			firstResponseTime = time.Since(startTime)
			assert.Equal(t, http.StatusOK, w.Code)

			var response handlers.WeatherResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, location, response.Location)
			assert.Equal(t, 22.0, response.Temperature)
		}()

		time.Sleep(3 * time.Second)

		startTime := time.Now()

		req := httptest.NewRequest("GET", "/weather?q="+location, nil)
		w := httptest.NewRecorder()

		ts.handler.GetWeather(w, req)

		secondResponseTime := time.Since(startTime)

		if secondResponseTime >= 3*time.Second {
			log.Error().Dur("elapsed", secondResponseTime).Msg("❌ TEST FAILED: Second request took too long")
			t.Fail()
		} else {
			log.Info().Dur("elapsed", secondResponseTime).Msg("✅ Second request completed in expected time")
		}

		wg.Wait()

		if firstResponseTime < 4*time.Second {
			log.Error().Dur("elapsed", firstResponseTime).Msg("❌ TEST FAILED: First request completed too quickly")
			t.Fail()
		} else {
			log.Info().Dur("elapsed", firstResponseTime).Msg("✅ First request held for expected time")
		}

		time.Sleep(100 * time.Millisecond)
		var query weatherquery.WeatherQuery
		result := db.Where("location = ?", location).Order("created_at DESC").First(&query)
		require.NoError(t, result.Error)

		if query.RequestCount != 2 {
			log.Error().Int("count", query.RequestCount).Msg("❌ TEST FAILED: Request count incorrect")
			t.Fail()
		} else {
			log.Info().Int("count", query.RequestCount).Msg("✅ Both requests aggregated correctly")
		}

		log.Info().Msg("✅ TEST PASSED: JoiningExistingQueue")
	})

	t.Run("MaxQueueSizeImmediateProcessing", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: MaxQueueSizeImmediateProcessing")

		ts := setupTest(t, 10*time.Second)
		defer ts.aggregator.Shutdown()

		location := "Sydney"

		ts.cacheProvider.On("Get", location).Return(nil, false, nil).Times(10)
		ts.weatherAPI.On("GetWeatherData", location).Return(28.5, 27.5, true, true, nil).Once()
		ts.cacheProvider.On("Set", location, mock.Anything, 30*time.Minute).Return(nil).Once()

		var wg sync.WaitGroup
		wg.Add(10)

		startTime := time.Now()

		log.Info().Msg("Launching 10 concurrent requests")

		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()

				req := httptest.NewRequest("GET", "/weather?q="+location, nil)
				w := httptest.NewRecorder()

				ts.handler.GetWeather(w, req)

				assert.Equal(t, http.StatusOK, w.Code)
			}()
		}

		wg.Wait()

		elapsedTime := time.Since(startTime)

		if elapsedTime >= 5*time.Second {
			log.Error().Dur("elapsed", elapsedTime).Msg("❌ TEST FAILED: Requests were not processed immediately")
			t.Fail()
		} else {
			log.Info().Dur("elapsed", elapsedTime).Msg("✅ Requests processed immediately when queue size reached max")
		}

		time.Sleep(100 * time.Millisecond)
		var query weatherquery.WeatherQuery
		result := db.Where("location = ?", location).Order("created_at DESC").First(&query)
		require.NoError(t, result.Error)

		if query.RequestCount != 10 {
			log.Error().Int("count", query.RequestCount).Msg("❌ TEST FAILED: Request count incorrect")
			t.Fail()
		} else {
			log.Info().Int("count", query.RequestCount).Msg("✅ Request count matches expected value")
		}

		log.Info().Msg("✅ TEST PASSED: MaxQueueSizeImmediateProcessing")
	})

	t.Run("BothAPIsSucceed", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: BothAPIsSucceed")

		ts := setupTest(t, 100*time.Millisecond)
		defer ts.aggregator.Shutdown()

		ts.cacheProvider.On("Get", "Paris").Return(nil, false, nil)
		ts.weatherAPI.On("GetWeatherData", "Paris").Return(23.5, 22.5, true, true, nil)
		ts.cacheProvider.On("Set", "Paris", mock.Anything, 30*time.Minute).Return(nil)

		req := httptest.NewRequest("GET", "/weather?q=Paris", nil)
		w := httptest.NewRecorder()

		ts.handler.GetWeather(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response handlers.WeatherResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		if response.Temperature != 23.0 {
			log.Error().Float64("temperature", response.Temperature).Msg("❌ TEST FAILED: Wrong temperature average")
			t.Fail()
		} else {
			log.Info().Float64("temperature", response.Temperature).Msg("✅ Temperature correctly averaged")
		}

		if response.Warning != "" {
			log.Error().Str("warning", response.Warning).Msg("❌ TEST FAILED: Should not have warning")
			t.Fail()
		} else {
			log.Info().Msg("✅ No warning as expected")
		}

		time.Sleep(100 * time.Millisecond)
		var query weatherquery.WeatherQuery
		result := db.Where("location = ?", "Paris").Order("created_at DESC").First(&query)
		require.NoError(t, result.Error)

		log.Info().Msg("✅ TEST PASSED: BothAPIsSucceed")
	})

	t.Run("OneAPISucceeds", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: OneAPISucceeds")

		ts := setupTest(t, 100*time.Millisecond)
		defer ts.aggregator.Shutdown()

		ts.cacheProvider.On("Get", "Rome").Return(nil, false, nil)
		ts.weatherAPI.On("GetWeatherData", "Rome").Return(26.5, 0.0, true, false, fmt.Errorf("API2 failed"))
		ts.cacheProvider.On("Set", "Rome", mock.Anything, 5*time.Minute).Return(nil)

		req := httptest.NewRequest("GET", "/weather?q=Rome", nil)
		w := httptest.NewRecorder()

		ts.handler.GetWeather(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response handlers.WeatherResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		if response.Temperature != 26.5 {
			log.Error().Float64("temperature", response.Temperature).Msg("❌ TEST FAILED: Wrong temperature")
			t.Fail()
		} else {
			log.Info().Float64("temperature", response.Temperature).Msg("✅ Temperature from first API used")
		}

		if response.Warning != "Using only first weather API data" {
			log.Error().Str("warning", response.Warning).Msg("❌ TEST FAILED: Incorrect warning")
			t.Fail()
		} else {
			log.Info().Str("warning", response.Warning).Msg("✅ Correct warning present")
		}

		time.Sleep(100 * time.Millisecond)
		var query weatherquery.WeatherQuery
		result := db.Where("location = ?", "Rome").Order("created_at DESC").First(&query)
		require.NoError(t, result.Error)

		log.Info().Msg("✅ TEST PASSED: OneAPISucceeds")
	})

	t.Run("BothAPIsFail", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: BothAPIsFail")

		ts := setupTest(t, 100*time.Millisecond)
		defer ts.aggregator.Shutdown()

		ts.cacheProvider.On("Get", "Tokyo").Return(nil, false, nil)
		ts.weatherAPI.On("GetWeatherData", "Tokyo").Return(0.0, 0.0, false, false, fmt.Errorf("both weather APIs failed"))

		req := httptest.NewRequest("GET", "/weather?q=Tokyo", nil)
		w := httptest.NewRecorder()

		ts.handler.GetWeather(w, req)

		if w.Code != http.StatusInternalServerError {
			log.Error().Int("status", w.Code).Msg("❌ TEST FAILED: Should return internal server error")
			t.Fail()
		} else {
			log.Info().Int("status", w.Code).Msg("✅ Returned correct error status")
		}

		var errorResponse handlers.ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)

		if len(errorResponse.Errors) != 1 || !strings.Contains(errorResponse.Errors[0].Detail, "both weather APIs failed") {
			log.Error().Interface("errors", errorResponse.Errors).Msg("❌ TEST FAILED: Incorrect error message")
			t.Fail()
		} else {
			log.Info().Msg("✅ Error message contains correct information")
		}

		log.Info().Msg("✅ TEST PASSED: BothAPIsFail")
	})

	t.Run("CachedResponse", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: CachedResponse")

		ts := setupTest(t, 5*time.Second)
		defer ts.aggregator.Shutdown()

		cachedData := &inmemorycache.WeatherCacheData{
			Temperature: 22.5,
			Warning:     "",
		}
		ts.cacheProvider.On("Get", "Berlin").Return(cachedData, true, nil)

		startTime := time.Now()

		req := httptest.NewRequest("GET", "/weather?q=Berlin", nil)
		w := httptest.NewRecorder()

		ts.handler.GetWeather(w, req)

		elapsedTime := time.Since(startTime)

		if elapsedTime >= 100*time.Millisecond {
			log.Error().Dur("elapsed", elapsedTime).Msg("❌ TEST FAILED: Cached response took too long")
			t.Fail()
		} else {
			log.Info().Dur("elapsed", elapsedTime).Msg("✅ Cached response returned immediately")
		}

		var response handlers.WeatherResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "Berlin", response.Location)
		assert.Equal(t, 22.5, response.Temperature)
		assert.Empty(t, response.Warning)

		ts.weatherAPI.AssertNotCalled(t, "GetWeatherData")

		log.Info().Msg("✅ TEST PASSED: CachedResponse")
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		t.Run("MissingLocation", func(t *testing.T) {
			log.Info().Msg("➡️ Running test: MissingLocation")

			ts := setupTest(t, 5*time.Second)
			defer ts.aggregator.Shutdown()

			req := httptest.NewRequest("GET", "/weather", nil)
			w := httptest.NewRecorder()

			ts.handler.GetWeather(w, req)

			if w.Code != http.StatusBadRequest {
				log.Error().Int("status", w.Code).Msg("❌ TEST FAILED: Should return bad request")
				t.Fail()
			} else {
				log.Info().Int("status", w.Code).Msg("✅ Returned correct error status")
			}

			var errorResponse handlers.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
			require.NoError(t, err)

			log.Info().Msg("✅ TEST PASSED: MissingLocation")
		})

		t.Run("InvalidMethod", func(t *testing.T) {
			log.Info().Msg("➡️ Running test: InvalidMethod")

			ts := setupTest(t, 5*time.Second)
			defer ts.aggregator.Shutdown()

			req := httptest.NewRequest("POST", "/weather?q=Madrid", nil)
			w := httptest.NewRecorder()

			ts.handler.GetWeather(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				log.Error().Int("status", w.Code).Msg("❌ TEST FAILED: Should return method not allowed")
				t.Fail()
			} else {
				log.Info().Int("status", w.Code).Msg("✅ Returned correct error status")
			}

			var errorResponse handlers.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
			require.NoError(t, err)

			log.Info().Msg("✅ TEST PASSED: InvalidMethod")
		})
	})

	t.Run("MultipleBatchesWithRequestCount", func(t *testing.T) {
		log.Info().Msg("➡️ Running test: MultipleBatchesWithRequestCount")

		ts := setupTest(t, 200*time.Millisecond)
		defer ts.aggregator.Shutdown()

		location := "Barcelona"

		ts.cacheProvider.On("Get", location).Return(nil, false, nil).Times(3)
		ts.weatherAPI.On("GetWeatherData", location).Return(22.5, 21.5, true, true, nil).Once()
		ts.cacheProvider.On("Set", location, mock.Anything, 30*time.Minute).Return(nil).Once()

		var wg1 sync.WaitGroup
		wg1.Add(3)

		for i := 0; i < 3; i++ {
			go func() {
				defer wg1.Done()
				req := httptest.NewRequest("GET", "/weather?q="+location, nil)
				w := httptest.NewRecorder()
				ts.handler.GetWeather(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}()
		}

		wg1.Wait()
		time.Sleep(300 * time.Millisecond)

		var query1 weatherquery.WeatherQuery
		result1 := db.Where("location = ?", location).Order("created_at DESC").First(&query1)
		require.NoError(t, result1.Error)

		if query1.RequestCount != 3 {
			log.Error().Int("count", query1.RequestCount).Msg("❌ TEST FAILED: First batch request count incorrect")
			t.Fail()
		} else {
			log.Info().Int("count", query1.RequestCount).Msg("✅ First batch count correct")
		}

		ts.cacheProvider.On("Get", location).Return(nil, false, nil).Times(5)
		ts.weatherAPI.On("GetWeatherData", location).Return(22.5, 21.5, true, true, nil).Once()
		ts.cacheProvider.On("Set", location, mock.Anything, 30*time.Minute).Return(nil).Once()

		var wg2 sync.WaitGroup
		wg2.Add(5)

		for i := 0; i < 5; i++ {
			go func() {
				defer wg2.Done()
				req := httptest.NewRequest("GET", "/weather?q="+location, nil)
				w := httptest.NewRecorder()
				ts.handler.GetWeather(w, req)
				assert.Equal(t, http.StatusOK, w.Code)
			}()
		}

		wg2.Wait()
		time.Sleep(300 * time.Millisecond)

		var query2 weatherquery.WeatherQuery
		result2 := db.Where("location = ?", location).Order("created_at DESC").First(&query2)
		require.NoError(t, result2.Error)

		if query2.RequestCount != 5 {
			log.Error().Int("count", query2.RequestCount).Msg("❌ TEST FAILED: Second batch request count incorrect")
			t.Fail()
		} else {
			log.Info().Int("count", query2.RequestCount).Msg("✅ Second batch count correct")
		}

		var entries []weatherquery.WeatherQuery
		db.Where("location = ?", location).Order("created_at DESC").Find(&entries)

		if len(entries) != 2 {
			log.Error().Int("entries", len(entries)).Msg("❌ TEST FAILED: Wrong number of database entries")
			t.Fail()
		} else {
			log.Info().Int("entries", len(entries)).Msg("✅ Correct number of database entries")
		}

		log.Info().Msg("✅ TEST PASSED: MultipleBatchesWithRequestCount")
	})
}
