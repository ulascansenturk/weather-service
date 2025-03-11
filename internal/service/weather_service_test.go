package service_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"ulascansenturk/weather-service/internal/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"ulascansenturk/weather-service/internal/service"
)

type WeatherServiceTestSuite struct {
	suite.Suite
	mockAggregator *mocks.MockWeatherRequestAggregator
	service        service.WeatherService
	ctx            context.Context
}

func (s *WeatherServiceTestSuite) SetupTest() {
	s.mockAggregator = mocks.NewMockWeatherRequestAggregator(s.T())
	s.service = service.NewWeatherService(s.mockAggregator)
	s.ctx = context.Background()
}

func convertToReceiveOnlyChannel(ch chan service.WeatherResponse) <-chan service.WeatherResponse {
	return ch
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithValidLocation() {
	location := "Istanbul"
	expectedResponse := service.WeatherResponse{
		Location:    location,
		Temperature: 25.5,
	}

	responseChan := make(chan service.WeatherResponse, 1)
	responseChan <- expectedResponse
	close(responseChan)

	s.mockAggregator.On("AddRequest", mock.Anything, location).
		Return(convertToReceiveOnlyChannel(responseChan), nil)

	result, err := s.service.GetWeather(s.ctx, location)

	s.NoError(err)
	s.Equal(expectedResponse, result)
	s.mockAggregator.AssertExpectations(s.T())
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithEmptyLocation() {
	result, err := s.service.GetWeather(s.ctx, "")

	s.Error(err)
	s.Equal(service.WeatherResponse{}, result)
	s.Contains(err.Error(), "location cannot be empty")

	s.mockAggregator.AssertNotCalled(s.T(), "AddRequest")
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithAggregatorError() {
	location := "Paris"
	expectedError := errors.New("aggregator error")

	s.mockAggregator.On("AddRequest", mock.Anything, location).
		Return((<-chan service.WeatherResponse)(nil), expectedError)

	result, err := s.service.GetWeather(s.ctx, location)

	s.Error(err)
	s.Equal(service.WeatherResponse{}, result)
	s.Equal(expectedError, err)
	s.mockAggregator.AssertExpectations(s.T())
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithErrorResponse() {
	location := "London"
	errorMsg := "weather API failed"

	errorResponse := service.WeatherResponse{
		Location: location,
		Error:    errorMsg,
	}

	responseChan := make(chan service.WeatherResponse, 1)
	responseChan <- errorResponse
	close(responseChan)

	s.mockAggregator.On("AddRequest", mock.Anything, location).
		Return(convertToReceiveOnlyChannel(responseChan), nil)

	result, err := s.service.GetWeather(s.ctx, location)

	s.Error(err)
	s.Equal(errorResponse, result)
	s.Contains(err.Error(), errorMsg)
	s.mockAggregator.AssertExpectations(s.T())
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithWarning() {
	location := "Berlin"
	warningMsg := "Using only API1 data"

	responseWithWarning := service.WeatherResponse{
		Location:    location,
		Temperature: 22.3,
		Warning:     warningMsg,
	}

	responseChan := make(chan service.WeatherResponse, 1)
	responseChan <- responseWithWarning
	close(responseChan)

	s.mockAggregator.On("AddRequest", mock.Anything, location).
		Return(convertToReceiveOnlyChannel(responseChan), nil)

	result, err := s.service.GetWeather(s.ctx, location)

	s.NoError(err)
	s.Equal(responseWithWarning, result)
	s.Equal(warningMsg, result.Warning)
	s.mockAggregator.AssertExpectations(s.T())
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithContextTimeout() {
	location := "Tokyo"

	ctx, cancel := context.WithTimeout(s.ctx, 50*time.Millisecond)
	defer cancel()

	responseChan := make(chan service.WeatherResponse)

	s.mockAggregator.On("AddRequest", mock.Anything, location).
		Return(convertToReceiveOnlyChannel(responseChan), nil)

	result, err := s.service.GetWeather(ctx, location)

	s.Error(err)
	s.Equal(service.WeatherResponse{}, result)
	s.Contains(err.Error(), "context deadline exceeded")
	s.mockAggregator.AssertExpectations(s.T())
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithCachedResult() {
	location := "Madrid"
	cachedTemp := 30.5

	cachedResponse := service.WeatherResponse{
		Location:    location,
		Temperature: cachedTemp,
	}

	responseChan := make(chan service.WeatherResponse, 1)
	responseChan <- cachedResponse
	close(responseChan)

	s.mockAggregator.On("AddRequest", mock.Anything, location).
		Return(convertToReceiveOnlyChannel(responseChan), nil)

	result, err := s.service.GetWeather(s.ctx, location)

	s.NoError(err)
	s.Equal(cachedResponse, result)
	s.Equal(cachedTemp, result.Temperature)
	s.mockAggregator.AssertExpectations(s.T())
}

func (s *WeatherServiceTestSuite) TestGetWeatherWithPartialSuccess() {
	location := "Sydney"

	partialResponse := service.WeatherResponse{
		Location:    location,
		Temperature: 28.7,
		Warning:     "Using only second weather API data",
	}

	responseChan := make(chan service.WeatherResponse, 1)
	responseChan <- partialResponse
	close(responseChan)

	s.mockAggregator.On("AddRequest", mock.Anything, location).
		Return(convertToReceiveOnlyChannel(responseChan), nil)

	result, err := s.service.GetWeather(s.ctx, location)

	s.NoError(err)
	s.Equal(partialResponse, result)
	s.Contains(result.Warning, "Using only second weather API data")
	s.mockAggregator.AssertExpectations(s.T())
}

func TestWeatherServiceSuite(t *testing.T) {
	suite.Run(t, new(WeatherServiceTestSuite))
}
