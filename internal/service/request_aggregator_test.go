package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"ulascansenturk/weather-service/internal/mocks"

	"github.com/stretchr/testify/suite"
	"ulascansenturk/weather-service/internal/service"
)

type WeatherAggregatorTestSuite struct {
	suite.Suite
	mockWeatherAPI    *mocks.MockWeatherAPIService
	mockRepo          *mocks.MockRepository
	weatherAggregator service.WeatherRequestAggregator
	ctx               context.Context
}

func (s *WeatherAggregatorTestSuite) SetupTest() {
	s.mockWeatherAPI = mocks.NewMockWeatherAPIService(s.T())
	s.mockRepo = mocks.NewMockRepository(s.T())

	s.weatherAggregator = service.NewWeatherRequestAggregator(
		s.mockWeatherAPI,
		s.mockRepo,
		10,
		5*time.Second,
	)

	s.ctx = context.Background()
}

func (s *WeatherAggregatorTestSuite) TestAddRequestWithNewQueue() {
	location := "Paris"

	responseChan, err := s.weatherAggregator.AddRequest(s.ctx, location)

	s.NoError(err)
	s.NotNil(responseChan)

	// Ensure no response has been sent yet
	select {
	case <-responseChan:
		s.Fail("Should not have received a response yet")
	default:
		// This is expected - no response yet
	}
}

func (s *WeatherAggregatorTestSuite) TestAddRequestWithMaxQueueSize() {
	location := "Tokyo"

	// Setup the mock for GetWeatherData
	s.mockWeatherAPI.On("GetWeatherData", location).Return(24.0, 26.0, true, true, nil)

	// The repository might be called to log the query
	s.mockRepo.On("LogWeatherQuery", location, 24.0, 26.0, 10).Return(nil).Maybe()

	var channels []<-chan service.WeatherResponse
	var wg sync.WaitGroup
	wg.Add(10)

	// Launch 10 concurrent requests to trigger max queue size
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			ch, err := s.weatherAggregator.AddRequest(s.ctx, location)
			s.NoError(err)
			if ch != nil {
				channels = append(channels, ch)
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Allow some time for processing queue
	time.Sleep(500 * time.Millisecond)

	// Since we're using a slice in a concurrent setting, we need to safely check
	// the response on each channel we captured
	for _, ch := range channels {
		select {
		case response, ok := <-ch:
			if !ok {
				s.Fail("Channel was closed without response")
				continue
			}
			s.Equal(location, response.Location)
			s.Equal(25.0, response.Temperature)
			s.Empty(response.Warning)
			s.Empty(response.Error)
		case <-time.After(500 * time.Millisecond):
			s.Fail("Did not receive response in time")
		}
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithBothAPIsSuccess() {
	location := "London"

	// Add a request to the queue
	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	// Setup the mock for GetWeatherData
	s.mockWeatherAPI.On("GetWeatherData", location).Return(18.0, 20.0, true, true, nil)

	// The repository might be called to log the query
	s.mockRepo.On("LogWeatherQuery", location, 18.0, 20.0, 1).Return(nil).Maybe()

	// Manually trigger processing by adding another request
	// In the refactored code, we don't have direct access to processQueue
	// We either need to wait for the timer or add max queue size
	// For testing, we'll force processing by simulating timeout
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.weatherAggregator.ProcessQueueForTesting(location)
	}()

	// Check that we get the expected response
	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(19.0, response.Temperature)
		s.Empty(response.Warning)
		s.Empty(response.Error)
	case <-time.After(500 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithOnlyAPI1Success() {
	location := "Berlin"

	// Add a request to the queue
	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	// Setup the mock for GetWeatherData with API1 success only
	s.mockWeatherAPI.On("GetWeatherData", location).Return(15.0, 0.0, true, false, errors.New("API2 failed"))

	// The repository might be called to log the query
	s.mockRepo.On("LogWeatherQuery", location, 15.0, 0.0, 1).Return(nil).Maybe()

	// Manually trigger processing
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.weatherAggregator.ProcessQueueForTesting(location)
	}()

	// Check that we get the expected response
	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(15.0, response.Temperature)
		s.Equal("Using only first weather API data", response.Warning)
		s.Empty(response.Error)
	case <-time.After(500 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithOnlyAPI2Success() {
	location := "Rome"

	// Add a request to the queue
	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	// Setup the mock for GetWeatherData with API2 success only
	s.mockWeatherAPI.On("GetWeatherData", location).Return(0.0, 22.0, false, true, errors.New("API1 failed"))

	// The repository might be called to log the query
	s.mockRepo.On("LogWeatherQuery", location, 0.0, 22.0, 1).Return(nil).Maybe()

	// Manually trigger processing
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.weatherAggregator.ProcessQueueForTesting(location)
	}()

	// Check that we get the expected response
	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(22.0, response.Temperature)
		s.Equal("Using only second weather API data", response.Warning)
		s.Empty(response.Error)
	case <-time.After(500 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithBothAPIsFailed() {
	location := "Moscow"
	expectedError := "Both APIs failed"

	// Add a request to the queue
	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	// Setup the mock for GetWeatherData with both APIs failed
	s.mockWeatherAPI.On("GetWeatherData", location).Return(0.0, 0.0, false, false, errors.New(expectedError))

	// Manually trigger processing
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.weatherAggregator.ProcessQueueForTesting(location)
	}()

	// Check that we get the expected error response
	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(0.0, response.Temperature)
		s.Empty(response.Warning)
		s.Equal(expectedError, response.Error)
	case <-time.After(500 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
	s.mockRepo.AssertNotCalled(s.T(), "LogWeatherQuery")
}

func (s *WeatherAggregatorTestSuite) TestShutdown() {
	location1 := "New York"
	location2 := "Chicago"

	// Add requests to the queue
	_, _ = s.weatherAggregator.AddRequest(s.ctx, location1)
	_, _ = s.weatherAggregator.AddRequest(s.ctx, location2)

	// Call shutdown
	s.weatherAggregator.Shutdown()

	// Setup the mock for a new request after shutdown
	s.mockWeatherAPI.On("GetWeatherData", location1).Return(25.0, 27.0, true, true, nil)
	s.mockRepo.On("LogWeatherQuery", location1, 25.0, 27.0, 1).Return(nil).Maybe()

	// Add a new request after shutdown (should create a new queue)
	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location1)

	// Manually trigger processing for the new request
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.weatherAggregator.ProcessQueueForTesting(location1)
	}()

	// Check that we get the expected response
	select {
	case response := <-responseChan:
		s.Equal(location1, response.Location)
		s.Equal(26.0, response.Temperature)
	case <-time.After(500 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func TestWeatherAggregatorSuite(t *testing.T) {
	suite.Run(t, new(WeatherAggregatorTestSuite))
}
