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

	select {
	case <-responseChan:
		s.Fail("Should not have received a response yet")
	default:
	}

}

func (s *WeatherAggregatorTestSuite) TestAddRequestWithMaxQueueSize() {
	location := "Tokyo"

	s.mockWeatherAPI.On("GetWeatherData", location).Return(24.0, 26.0, true, true, nil)

	s.mockRepo.On("LogWeatherQuery", location, 24.0, 26.0, 10).Return(nil).Maybe()

	var channels []<-chan service.WeatherResponse
	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			ch, err := s.weatherAggregator.AddRequest(s.ctx, location)
			s.NoError(err)
			channels = append(channels, ch)
		}()
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	for _, ch := range channels {
		select {
		case response := <-ch:
			s.Equal(location, response.Location)
			s.Equal(25.0, response.Temperature)
			s.Empty(response.Warning)
			s.Empty(response.Error)
		case <-time.After(100 * time.Millisecond):
			s.Fail("Did not receive response in time")
		}
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithBothAPIsSuccess() {
	location := "London"

	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	s.mockWeatherAPI.On("GetWeatherData", location).Return(18.0, 20.0, true, true, nil)

	s.mockRepo.On("LogWeatherQuery", location, 18.0, 20.0, 1).Return(nil).Maybe()

	s.weatherAggregator.ProcessQueue(location)

	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(19.0, response.Temperature)
		s.Empty(response.Warning)
		s.Empty(response.Error)
	case <-time.After(100 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	time.Sleep(100 * time.Millisecond)

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithOnlyAPI1Success() {
	location := "Berlin"

	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	s.mockWeatherAPI.On("GetWeatherData", location).Return(15.0, 0.0, true, false, errors.New("API2 failed"))

	s.mockRepo.On("LogWeatherQuery", location, 15.0, 0.0, 1).Return(nil).Maybe()

	s.weatherAggregator.ProcessQueue(location)

	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(15.0, response.Temperature)
		s.Equal("Using only first weather API data", response.Warning)
		s.Empty(response.Error)
	case <-time.After(100 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	time.Sleep(100 * time.Millisecond)

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithOnlyAPI2Success() {
	location := "Rome"

	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	s.mockWeatherAPI.On("GetWeatherData", location).Return(0.0, 22.0, false, true, errors.New("API1 failed"))

	s.mockRepo.On("LogWeatherQuery", location, 0.0, 22.0, 1).Return(nil).Maybe()

	s.weatherAggregator.ProcessQueue(location)

	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(22.0, response.Temperature)
		s.Equal("Using only second weather API data", response.Warning)
		s.Empty(response.Error)
	case <-time.After(100 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	time.Sleep(100 * time.Millisecond)

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func (s *WeatherAggregatorTestSuite) TestProcessQueueWithBothAPIsFailed() {
	location := "Moscow"
	expectedError := "Both APIs failed"

	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location)

	s.mockWeatherAPI.On("GetWeatherData", location).Return(0.0, 0.0, false, false, errors.New(expectedError))

	s.weatherAggregator.ProcessQueue(location)

	select {
	case response := <-responseChan:
		s.Equal(location, response.Location)
		s.Equal(0.0, response.Temperature)
		s.Empty(response.Warning)
		s.Equal(expectedError, response.Error)
	case <-time.After(100 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	s.mockWeatherAPI.AssertExpectations(s.T())
	s.mockRepo.AssertNotCalled(s.T(), "LogWeatherQuery")
}

func (s *WeatherAggregatorTestSuite) TestShutdown() {
	location1 := "New York"
	location2 := "Chicago"

	_, _ = s.weatherAggregator.AddRequest(s.ctx, location1)
	_, _ = s.weatherAggregator.AddRequest(s.ctx, location2)

	s.weatherAggregator.Shutdown()

	s.mockWeatherAPI.On("GetWeatherData", location1).Return(25.0, 27.0, true, true, nil)

	s.mockRepo.On("LogWeatherQuery", location1, 25.0, 27.0, 1).Return(nil).Maybe()

	responseChan, _ := s.weatherAggregator.AddRequest(s.ctx, location1)
	s.weatherAggregator.ProcessQueue(location1)

	select {
	case response := <-responseChan:
		s.Equal(location1, response.Location)
		s.Equal(26.0, response.Temperature)
	case <-time.After(100 * time.Millisecond):
		s.Fail("Did not receive response in time")
	}

	time.Sleep(100 * time.Millisecond)

	s.mockWeatherAPI.AssertExpectations(s.T())
}

func TestWeatherAggregatorSuite(t *testing.T) {
	suite.Run(t, new(WeatherAggregatorTestSuite))
}
