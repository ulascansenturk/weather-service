package service

import (
	"context"
	"errors"
)

type WeatherResponse struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Warning     string  `json:"warning,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type WeatherService interface {
	GetWeather(ctx context.Context, location string) (WeatherResponse, error)
}

type weatherService struct {
	aggregator WeatherRequestAggregator
}

func NewWeatherService(aggregator WeatherRequestAggregator) WeatherService {
	return &weatherService{
		aggregator: aggregator,
	}
}

func (s *weatherService) GetWeather(ctx context.Context, location string) (WeatherResponse, error) {
	if location == "" {
		return WeatherResponse{}, errors.New("location cannot be empty")
	}

	responseChan, err := s.aggregator.AddRequest(ctx, location)
	if err != nil {
		return WeatherResponse{}, err
	}

	select {
	case response := <-responseChan:
		if response.Error != "" {
			return response, errors.New(response.Error)
		}
		return response, nil
	case <-ctx.Done():
		return WeatherResponse{}, ctx.Err()
	}
}
