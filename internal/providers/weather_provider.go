package providers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type WeatherAPIService interface {
	GetWeatherFromWeatherAPI(location string) (float64, error)
	GetWeatherFromAPIWeatherStackAPI(location string) (float64, error)
	GetWeatherData(location string) (float64, float64, bool, bool, error)
	GetHTTPClient() *http.Client
}

type weatherAPIService struct {
	api1Key string
	api2Key string
	client  *http.Client
}

func NewWeatherAPIService(api1Key, api2Key string) WeatherAPIService {
	return &weatherAPIService{
		api1Key: api1Key,
		api2Key: api2Key,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type WeatherAPI1Response struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type WeatherAPI2Response struct {
	Current struct {
		Temperature float64 `json:"temperature"`
	} `json:"current"`
	Success bool `json:"success"`
	Error   struct {
		Code int    `json:"code"`
		Type string `json:"type"`
		Info string `json:"info"`
	} `json:"error,omitempty"`
}

// TODO: move base url to config
func (s *weatherAPIService) GetWeatherFromWeatherAPI(location string) (float64, error) {
	url := fmt.Sprintf("http://api.weatherapi.com/v1/forecast.json?key=%s&q=%s&days=1&aqi=no&alerts=no", s.api1Key, location)

	resp, err := s.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("API1 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API1 returned status code: %d", resp.StatusCode)
	}

	var apiResp WeatherAPI1Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("API1 returned malformed JSON: %w", err)
	}

	if apiResp.Error.Code != 0 {
		return 0, fmt.Errorf("API1 error: %s (code %d)", apiResp.Error.Message, apiResp.Error.Code)
	}

	if apiResp.Current.TempC < -100 || apiResp.Current.TempC > 100 {
		return 0, fmt.Errorf("API1 returned unlikely temperature value: %f", apiResp.Current.TempC)
	}

	return apiResp.Current.TempC, nil
}

// TODO: move base url to config
func (s *weatherAPIService) GetWeatherFromAPIWeatherStackAPI(location string) (float64, error) {
	url := fmt.Sprintf("http://api.weatherstack.com/current?access_key=%s&query=%s", s.api2Key, location)

	resp, err := s.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("API2 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API2 returned status code: %d", resp.StatusCode)
	}

	var apiResp WeatherAPI2Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("API2 returned malformed JSON: %w", err)
	}

	// Weatherstack has a success field we can check
	if !apiResp.Success && apiResp.Error.Code != 0 {
		return 0, fmt.Errorf("API2 error: %s (code %d, type %s)",
			apiResp.Error.Info, apiResp.Error.Code, apiResp.Error.Type)
	}

	return apiResp.Current.Temperature, nil
}

func (s *weatherAPIService) GetWeatherData(location string) (float64, float64, bool, bool, error) {
	var service1Temp, service2Temp float64
	var err1, err2 error

	ch1 := make(chan struct {
		temp float64
		err  error
	})
	ch2 := make(chan struct {
		temp float64
		err  error
	})

	go func() {
		temp, err := s.GetWeatherFromWeatherAPI(location)
		ch1 <- struct {
			temp float64
			err  error
		}{temp, err}
	}()

	go func() {
		temp, err := s.GetWeatherFromAPIWeatherStackAPI(location)
		ch2 <- struct {
			temp float64
			err  error
		}{temp, err}
	}()

	res1 := <-ch1
	res2 := <-ch2

	service1Temp, err1 = res1.temp, res1.err
	service2Temp, err2 = res2.temp, res2.err

	api1Success := (err1 == nil)
	api2Success := (err2 == nil)

	var resultErr error
	if !api1Success && !api2Success {
		resultErr = fmt.Errorf("both weather APIs failed - API1: %v, API2: %v", err1, err2)
	} else if !api1Success {
		resultErr = fmt.Errorf("API1 failed: %v", err1)
	} else if !api2Success {
		resultErr = fmt.Errorf("API2 failed: %v", err2)
	}

	return service1Temp, service2Temp, api1Success, api2Success, resultErr
}

func (s *weatherAPIService) GetHTTPClient() *http.Client {
	return s.client
}
