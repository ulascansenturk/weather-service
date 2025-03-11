package service

import (
	"context"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
	"ulascansenturk/weather-service/internal/db/weatherquery"
	"ulascansenturk/weather-service/internal/providers"
)

type WeatherRequestAggregator interface {
	AddRequest(ctx context.Context, location string) (<-chan WeatherResponse, error)
	ProcessQueueForTesting(location string)
	Shutdown()
}

type locationQueue struct {
	channels []chan WeatherResponse
	timer    *time.Timer
	mu       sync.Mutex
}

type weatherAggregator struct {
	weatherAPI       providers.WeatherAPIService
	weatherQueryRepo weatherquery.Repository
	queues           map[string]*locationQueue // init map to sure it is not nil
	queueMutex       sync.RWMutex              // Use RWMutex to allow concurrent reads
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
		queues:           make(map[string]*locationQueue),
		maxQueueSize:     maxQueueSize,
		maxWaitTime:      maxWaitTime,
	}
}

func (w *weatherAggregator) AddRequest(ctx context.Context, location string) (<-chan WeatherResponse, error) {
	// init buffer channel with size 1 to avoid blocking
	responseChan := make(chan WeatherResponse, 1)

	w.queueMutex.RLock()
	queue, exists := w.queues[location]
	w.queueMutex.RUnlock()

	//check if queue exist for location
	if !exists {
		w.queueMutex.Lock()
		//double check if queue is created by another goroutine. better safe than sorry
		queue, exists = w.queues[location]
		if !exists {
			queue = &locationQueue{}
			w.queues[location] = queue
		}
		w.queueMutex.Unlock()
	}

	// lock the specific location queue
	queue.mu.Lock()
	defer queue.mu.Unlock()

	// if its the first request for the location, start the timer (max 5 seconds)
	if len(queue.channels) == 0 {
		queue.timer = time.AfterFunc(w.maxWaitTime, func() {
			w.processQueue(location)
		})
	}

	// add the response chhanel to queue
	queue.channels = append(queue.channels, responseChan)

	if len(queue.channels) >= w.maxQueueSize {
		// if we hit the max limit no need to wait for timer, process directly
		if queue.timer != nil {
			queue.timer.Stop()
			queue.timer = nil
		}
		go w.processQueue(location)
	}

	return responseChan, nil
}

func (w *weatherAggregator) processQueue(location string) {
	var channels []chan WeatherResponse

	w.queueMutex.RLock()
	queue, exists := w.queues[location]
	w.queueMutex.RUnlock()

	if !exists {
		return
	}

	queue.mu.Lock()

	if len(queue.channels) == 0 {
		queue.mu.Unlock()
		return
	}

	channels = queue.channels
	queue.channels = nil

	if queue.timer != nil {
		queue.timer.Stop()
		queue.timer = nil
	}

	queue.mu.Unlock()

	w.queueMutex.Lock()
	delete(w.queues, location)
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

		return
	}

	go func() {
		if w.weatherQueryRepo != nil {
			if err := w.weatherQueryRepo.LogWeatherQuery(location, service1Temp, service2Temp, len(channels)); err != nil {
				log.Log().Err(err).Msg("Failed to log weather query")
			}
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
}

func (w *weatherAggregator) Shutdown() {
	w.queueMutex.Lock()
	defer w.queueMutex.Unlock()

	for _, queue := range w.queues {
		queue.mu.Lock()

		if queue.timer != nil {
			queue.timer.Stop()
		}

		for _, ch := range queue.channels {
			close(ch)
		}

		queue.mu.Unlock()
	}

	w.queues = make(map[string]*locationQueue)
}

func (w *weatherAggregator) ProcessQueueForTesting(location string) {
	w.processQueue(location)
}
