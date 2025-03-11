package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"ulascansenturk/weather-service/internal/api/v1/handlers"
	"ulascansenturk/weather-service/internal/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"ulascansenturk/weather-service/internal/service"
)

type WeatherHandlerTestSuite struct {
	suite.Suite
	mockService *mocks.MockWeatherService
	handler     *handlers.WeatherHandler
}

func (s *WeatherHandlerTestSuite) SetupTest() {
	s.mockService = mocks.NewMockWeatherService(s.T())
	s.handler = handlers.NewWeatherHandler(s.mockService, 5*time.Second)
}

func (s *WeatherHandlerTestSuite) TestHandleWeatherRequestSuccess() {
	location := "Istanbul"
	temperature := 25.5

	s.mockService.On("GetWeather", mock.Anything, location).Return(
		service.WeatherResponse{
			Location:    location,
			Temperature: temperature,
		},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/weather?q="+location, nil)
	recorder := httptest.NewRecorder()

	s.handler.GetWeather(recorder, req)

	s.Equal(http.StatusOK, recorder.Code)

	var response handlers.WeatherResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	s.NoError(err)
	s.Equal(location, response.Location)
	s.Equal(temperature, response.Temperature)
	s.Empty(response.Warning)

	s.mockService.AssertExpectations(s.T())
}

func (s *WeatherHandlerTestSuite) TestHandleWeatherRequestWithWarning() {
	location := "Paris"
	temperature := 22.5
	warning := "Using only API1 data"

	s.mockService.On("GetWeather", mock.Anything, location).Return(
		service.WeatherResponse{
			Location:    location,
			Temperature: temperature,
			Warning:     warning,
		},
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/weather?q="+location, nil)
	recorder := httptest.NewRecorder()

	s.handler.GetWeather(recorder, req)

	s.Equal(http.StatusOK, recorder.Code)

	var response handlers.WeatherResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	s.NoError(err)
	s.Equal(location, response.Location)
	s.Equal(temperature, response.Temperature)
	s.Equal(warning, response.Warning)

	s.mockService.AssertExpectations(s.T())
}

func (s *WeatherHandlerTestSuite) TestHandleWeatherRequestMissingLocation() {
	req := httptest.NewRequest(http.MethodGet, "/weather", nil)
	recorder := httptest.NewRecorder()

	s.handler.GetWeather(recorder, req)

	s.Equal(http.StatusBadRequest, recorder.Code)

	var response handlers.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	s.NoError(err)
	s.Len(response.Errors, 1)
	s.Equal("BAD_REQUEST", response.Errors[0].Code)
	s.Contains(response.Errors[0].Detail, "location parameter")

	s.mockService.AssertNotCalled(s.T(), "GetWeather")
}

func (s *WeatherHandlerTestSuite) TestHandleWeatherRequestWrongMethod() {
	req := httptest.NewRequest(http.MethodPost, "/weather?q=Istanbul", nil)
	recorder := httptest.NewRecorder()

	s.handler.GetWeather(recorder, req)

	s.Equal(http.StatusMethodNotAllowed, recorder.Code)

	var response handlers.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	s.NoError(err)
	s.Len(response.Errors, 1)
	s.Equal("METHOD_NOT_ALLOWED", response.Errors[0].Code)
	s.Contains(response.Errors[0].Detail, "method not allowed")

	s.mockService.AssertNotCalled(s.T(), "GetWeather")
}

func (s *WeatherHandlerTestSuite) TestHandleWeatherRequestWrongPath() {
	req := httptest.NewRequest(http.MethodGet, "/forecast?q=Istanbul", nil)
	recorder := httptest.NewRecorder()

	s.handler.GetWeather(recorder, req)

	s.Equal(http.StatusNotFound, recorder.Code)

	var response handlers.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	s.NoError(err)
	s.Len(response.Errors, 1)
	s.Equal("NOT_FOUND", response.Errors[0].Code)
	s.Contains(response.Errors[0].Detail, "not found")

	s.mockService.AssertNotCalled(s.T(), "GetWeather")
}

func (s *WeatherHandlerTestSuite) TestHandleWeatherRequestServiceError() {
	location := "InvalidCity"
	expectedError := errors.New("service error")

	s.mockService.On("GetWeather", mock.Anything, location).Return(
		service.WeatherResponse{},
		expectedError,
	)

	req := httptest.NewRequest(http.MethodGet, "/weather?q="+location, nil)
	recorder := httptest.NewRecorder()

	s.handler.GetWeather(recorder, req)

	s.Equal(http.StatusInternalServerError, recorder.Code)

	var response handlers.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	s.NoError(err)
	s.Len(response.Errors, 1)
	s.Equal("INTERNAL_ERROR", response.Errors[0].Code)
	s.Contains(response.Errors[0].Detail, expectedError.Error())

	s.mockService.AssertExpectations(s.T())
}

func (s *WeatherHandlerTestSuite) TestHandleWeatherRequestContextTimeout() {
	location := "SlowCity"

	// Set up the mock to block until context is canceled
	s.mockService.On("GetWeather", mock.Anything, location).
		Run(func(args mock.Arguments) {
			ctx := args.Get(0).(context.Context)
			<-ctx.Done() // Block until context is canceled
		}).
		Return(service.WeatherResponse{}, context.DeadlineExceeded)

	// Use a very short timeout for the test
	s.handler = handlers.NewWeatherHandler(s.mockService, 50*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/weather?q="+location, nil)
	recorder := httptest.NewRecorder()

	s.handler.GetWeather(recorder, req)

	s.Equal(http.StatusInternalServerError, recorder.Code)

	var response handlers.ErrorResponse
	err := json.NewDecoder(recorder.Body).Decode(&response)
	s.NoError(err)
	s.Len(response.Errors, 1)
	s.Equal("INTERNAL_ERROR", response.Errors[0].Code)
	s.Contains(response.Errors[0].Detail, "context deadline exceeded")

	s.mockService.AssertExpectations(s.T())
}

func TestWeatherHandlerSuite(t *testing.T) {
	suite.Run(t, new(WeatherHandlerTestSuite))
}
