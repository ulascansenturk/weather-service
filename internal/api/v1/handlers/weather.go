package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"ulascansenturk/weather-service/internal/service"
)

type WeatherHandler struct {
	weatherService service.WeatherService
	timeout        time.Duration
}

func NewWeatherHandler(weatherService service.WeatherService, timeout time.Duration) *WeatherHandler {
	return &WeatherHandler{
		weatherService: weatherService,
		timeout:        timeout,
	}
}

func (h *WeatherHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/weather":
		h.GetWeather(w, r)
	default:
		respondWithError(w, http.StatusNotFound, "not found")
	}
}

func (h *WeatherHandler) GetWeather(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		respondWithError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if r.URL.Path != "/weather" {
		respondWithError(w, http.StatusNotFound, "not found")
		return
	}

	location := r.URL.Query().Get("q")
	if location == "" {
		respondWithError(w, http.StatusBadRequest, "location parameter 'q' is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	response, err := h.weatherService.GetWeather(ctx, location)
	if err != nil {
		log.Error().Err(err).Str("location", location).Msg("failed to get weather data")
		respondWithError(w, http.StatusInternalServerError, "failed to get weather data: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, WeatherResponse{
		Location:    location,
		Temperature: response.Temperature,
		Warning:     response.Warning,
	})
}
