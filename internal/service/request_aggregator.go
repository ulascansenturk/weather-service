package service

import (
	"context"
	"sync"
	"time"
	"ulascansenturk/weather-service/internal/db/weatherquery"
	"ulascansenturk/weather-service/internal/providers"
)

type WeatherRequestAggregator interface {
	AddRequest(ctx context.Context, location string) (<-chan WeatherResponse, error)
	ProcessQueue(location string)
	Shutdown()
}

type weatherAggregator struct {
	weatherAPI       providers.WeatherAPIService
	weatherQueryRepo weatherquery.Repository
	queues           map[string]*RequestQueue
	queueMutex       sync.Mutex
	maxQueueSize     int
	maxWaitTime      time.Duration
}

func NewWeatherRequestAggregator(
	weatherAPI providers.WeatherAPIService,
	weatherQueryRepo weatherquery.Repository,
	maxQueueSize int,
	maxWaitTime time.Duration,
) WeatherRequestAggregator {
	return &weatherAggregator{
		weatherAPI:       weatherAPI,
		weatherQueryRepo: weatherQueryRepo,
		queues:           make(map[string]*RequestQueue),
		maxQueueSize:     maxQueueSize,
		maxWaitTime:      maxWaitTime,
	}
}

func (w *weatherAggregator) AddRequest(ctx context.Context, location string) (<-chan WeatherResponse, error) {
	responseChan := make(chan WeatherResponse, 1)
	shouldProcess := false

	w.queueMutex.Lock()
	defer w.queueMutex.Unlock()

	if _, exists := w.queues[location]; !exists {
		w.queues[location] = &RequestQueue{
			Channels:   make([]chan WeatherResponse, 0),
			CreatedAt:  time.Now(),
			InProgress: false,
		}

		w.queues[location].Timer = time.AfterFunc(w.maxWaitTime, func() {
			w.ProcessQueue(location)
		})
		shouldProcess = false
	} else if w.queues[location].InProgress {
		w.queues[location].Timer.Stop()
		w.queues[location] = &RequestQueue{
			Channels:   make([]chan WeatherResponse, 0),
			CreatedAt:  time.Now(),
			InProgress: false,
		}
		w.queues[location].Timer = time.AfterFunc(w.maxWaitTime, func() {
			w.ProcessQueue(location)
		})
		shouldProcess = false
	}

	w.queues[location].Channels = append(w.queues[location].Channels, responseChan)

	if len(w.queues[location].Channels) >= w.maxQueueSize {
		w.queues[location].Timer.Stop()
		shouldProcess = true
		w.queues[location].InProgress = true
	}

	if shouldProcess {
		go w.ProcessQueue(location)
	}

	return responseChan, nil
}

func (w *weatherAggregator) ProcessQueue(location string) {
	w.queueMutex.Lock()

	queue, exists := w.queues[location]
	if !exists || len(queue.Channels) == 0 {
		w.queueMutex.Unlock()
		return
	}

	queue.InProgress = true
	channels := queue.Channels
	queue.Channels = nil

	w.queueMutex.Unlock()

	service1Temp, service2Temp, api1Success, api2Success, apiErr := w.weatherAPI.GetWeatherData(location)

	var avgTemp float64
	var warningMsg string

	if api1Success && api2Success {
		avgTemp = (service1Temp + service2Temp) / 2
	} else if api1Success {
		avgTemp = service1Temp
		warningMsg = "Using only first weather API data"
	} else if api2Success {
		avgTemp = service2Temp
		warningMsg = "Using only second weather API data"
	} else {
		errMsg := "All weather API services failed"
		if apiErr != nil {
			errMsg = apiErr.Error()
		}

		for _, ch := range channels {
			ch <- WeatherResponse{
				Location: location,
				Error:    errMsg,
			}
			close(ch)
		}

		w.queueMutex.Lock()
		delete(w.queues, location)
		w.queueMutex.Unlock()

		return
	}

	go func() {
		if w.weatherQueryRepo != nil {
			_ = w.weatherQueryRepo.LogWeatherQuery(location, service1Temp, service2Temp, len(channels))
		}
	}()

	for _, ch := range channels {
		ch <- WeatherResponse{
			Location:    location,
			Temperature: avgTemp,
			Warning:     warningMsg,
		}
		close(ch)
	}

	w.queueMutex.Lock()
	delete(w.queues, location)
	w.queueMutex.Unlock()
}

func (w *weatherAggregator) Shutdown() {
	w.queueMutex.Lock()
	defer w.queueMutex.Unlock()

	for _, queue := range w.queues {
		if queue.Timer != nil {
			queue.Timer.Stop()
		}

		for _, ch := range queue.Channels {
			close(ch)
		}
	}

	w.queues = make(map[string]*RequestQueue)
}
