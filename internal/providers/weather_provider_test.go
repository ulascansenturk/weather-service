package providers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"ulascansenturk/weather-service/internal/providers"

	"github.com/stretchr/testify/suite"
)

type WeatherAPIServiceTestSuite struct {
	suite.Suite
	api1Server *httptest.Server
	api2Server *httptest.Server
	service    providers.WeatherAPIService
}

func (s *WeatherAPIServiceTestSuite) SetupTest() {
	s.api1Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location := r.URL.Query().Get("q")
		switch location {
		case "ValidCity":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"current": map[string]interface{}{
					"temp_c": 25.5,
				},
			})
		case "ErrorCity":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    1006,
					"message": "No matching location found",
				},
			})
		case "MalformedJSON":
			w.Write([]byte("{malformed json"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	s.api2Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		location := r.URL.Query().Get("query")
		switch location {
		case "ValidCity":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"current": map[string]interface{}{
					"temperature": 26.5,
				},
			})
		case "ErrorCity":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error": map[string]interface{}{
					"code": 615,
					"type": "request_failed",
					"info": "Your API request failed",
				},
			})
		case "InvalidTemp":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"current": map[string]interface{}{
					"temperature": -150.0,
				},
			})
		case "MalformedJSON":
			w.Write([]byte("{malformed json"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))

	api1Key := "test_api1_key"
	api2Key := "test_api2_key"
	s.service = providers.NewWeatherAPIService(api1Key, api2Key)

	httpClient := s.service.GetHTTPClient()
	httpClient.Transport = &mockTransport{
		api1URL: s.api1Server.URL,
		api2URL: s.api2Server.URL,
	}
}

func (s *WeatherAPIServiceTestSuite) TearDownTest() {
	s.api1Server.Close()
	s.api2Server.Close()
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI1_Success() {
	temp, err := s.service.GetWeatherFromWeatherAPI("ValidCity")
	s.NoError(err)
	s.Equal(25.5, temp)
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI1_Error() {
	_, err := s.service.GetWeatherFromWeatherAPI("ErrorCity")
	s.Error(err)
	s.Contains(err.Error(), "No matching location found")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI1_MalformedJSON() {
	_, err := s.service.GetWeatherFromWeatherAPI("MalformedJSON")
	s.Error(err)
	s.Contains(err.Error(), "malformed JSON")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI1_ServerError() {
	_, err := s.service.GetWeatherFromWeatherAPI("ServerError")
	s.Error(err)
	s.Contains(err.Error(), "status code")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI2_Success() {
	temp, err := s.service.GetWeatherFromAPIWeatherStackAPI("ValidCity")
	s.NoError(err)
	s.Equal(26.5, temp)
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI2_Error() {
	_, err := s.service.GetWeatherFromAPIWeatherStackAPI("ErrorCity")
	s.Error(err)
	s.Contains(err.Error(), "Your API request failed")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI2_MalformedJSON() {
	_, err := s.service.GetWeatherFromAPIWeatherStackAPI("MalformedJSON")
	s.Error(err)
	s.Contains(err.Error(), "malformed JSON")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherFromAPI2_ServerError() {
	_, err := s.service.GetWeatherFromAPIWeatherStackAPI("ServerError")
	s.Error(err)
	s.Contains(err.Error(), "status code")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherData_BothSuccess() {
	temp1, temp2, api1Success, api2Success, err := s.service.GetWeatherData("ValidCity")
	s.NoError(err)
	s.True(api1Success)
	s.True(api2Success)
	s.Equal(25.5, temp1)
	s.Equal(26.5, temp2)
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherData_BothFail() {
	temp1, temp2, api1Success, api2Success, err := s.service.GetWeatherData("ServerError")
	s.Error(err)
	s.False(api1Success)
	s.False(api2Success)
	s.Equal(0.0, temp1)
	s.Equal(0.0, temp2)
	s.Contains(err.Error(), "both weather APIs failed")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherData_API1FailsOnly() {
	s.api1Server.Close()
	s.api1Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	httpClient := s.service.GetHTTPClient()
	httpClient.Transport = &mockTransport{
		api1URL: s.api1Server.URL,
		api2URL: s.api2Server.URL,
	}

	temp1, temp2, api1Success, api2Success, err := s.service.GetWeatherData("ValidCity")
	s.Error(err)
	s.False(api1Success)
	s.True(api2Success)
	s.Equal(0.0, temp1)
	s.Equal(26.5, temp2)
	s.Contains(err.Error(), "API1 failed")
}

func (s *WeatherAPIServiceTestSuite) TestGetWeatherData_API2FailsOnly() {
	s.api2Server.Close()
	s.api2Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	httpClient := s.service.GetHTTPClient()
	httpClient.Transport = &mockTransport{
		api1URL: s.api1Server.URL,
		api2URL: s.api2Server.URL,
	}

	temp1, temp2, api1Success, api2Success, err := s.service.GetWeatherData("ValidCity")
	s.Error(err)
	s.True(api1Success)
	s.False(api2Success)
	s.Equal(25.5, temp1)
	s.Equal(0.0, temp2)
	s.Contains(err.Error(), "API2 failed")
}

type mockTransport struct {
	api1URL string
	api2URL string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "api.weatherapi.com" {
		newURL := *req.URL
		newURL.Scheme = "http"
		newURL.Host = m.api1URL[7:] // Remove "http://"
		req.URL = &newURL
		return http.DefaultTransport.RoundTrip(req)
	} else if req.URL.Host == "api.weatherstack.com" {
		newURL := *req.URL
		newURL.Scheme = "http"
		newURL.Host = m.api2URL[7:] // Remove "http://"
		req.URL = &newURL
		return http.DefaultTransport.RoundTrip(req)
	}

	return http.DefaultTransport.RoundTrip(req)
}

func TestWeatherAPIServiceSuite(t *testing.T) {
	suite.Run(t, new(WeatherAPIServiceTestSuite))
}
