package inmemorycache_test

import (
	"testing"
	"time"
	"ulascansenturk/weather-service/internal/inmemorycache"

	"github.com/stretchr/testify/suite"
)

type InMemoryCacheTestSuite struct {
	suite.Suite
	cacheProvider *inmemorycache.InMemoryCache
}

func (s *InMemoryCacheTestSuite) SetupTest() {
	s.cacheProvider = inmemorycache.NewInMemoryCacheProvider(100 * time.Millisecond)
}

func (s *InMemoryCacheTestSuite) TestGetNonExistentKey() {
	value, exists, err := s.cacheProvider.Get("nonexistent")

	s.NoError(err)
	s.False(exists)
	s.Nil(value)
}

func (s *InMemoryCacheTestSuite) TestSetAndGetTemperature() {
	location := "Paris"
	temperature := 22.5
	ttl := 5 * time.Minute

	cacheData := &inmemorycache.WeatherCacheData{
		Temperature: temperature,
	}

	err := s.cacheProvider.Set(location, cacheData, ttl)
	s.NoError(err)

	value, exists, err := s.cacheProvider.Get(location)
	s.NoError(err)
	s.True(exists)
	s.NotNil(value)
	s.Equal(temperature, value.Temperature)
	s.Empty(value.Warning)
}

func (s *InMemoryCacheTestSuite) TestSetAndGetWithWarning() {
	location := "Berlin"
	temperature := 15.0
	warning := "Using only first weather API data"
	ttl := 5 * time.Minute

	cacheData := &inmemorycache.WeatherCacheData{
		Temperature: temperature,
		Warning:     warning,
	}

	err := s.cacheProvider.Set(location, cacheData, ttl)
	s.NoError(err)

	value, exists, err := s.cacheProvider.Get(location)
	s.NoError(err)
	s.True(exists)
	s.NotNil(value)
	s.Equal(temperature, value.Temperature)
	s.Equal(warning, value.Warning)
}

func (s *InMemoryCacheTestSuite) TestExpiration() {
	location := "Berlin"
	temperature := 15.0
	ttl := 50 * time.Millisecond

	cacheData := &inmemorycache.WeatherCacheData{
		Temperature: temperature,
	}

	err := s.cacheProvider.Set(location, cacheData, ttl)
	s.NoError(err)

	value, exists, err := s.cacheProvider.Get(location)
	s.NoError(err)
	s.True(exists)
	s.NotNil(value)
	s.Equal(temperature, value.Temperature)

	time.Sleep(75 * time.Millisecond)

	value, exists, err = s.cacheProvider.Get(location)
	s.NoError(err)
	s.False(exists)
	s.Nil(value)
}

func (s *InMemoryCacheTestSuite) TestOverwrite() {
	location := "Rome"
	temperature1 := 25.0
	temperature2 := 27.5
	warning1 := "Using only first weather API data"
	warning2 := "Using only second weather API data"
	ttl := 5 * time.Minute

	cacheData1 := &inmemorycache.WeatherCacheData{
		Temperature: temperature1,
		Warning:     warning1,
	}

	err := s.cacheProvider.Set(location, cacheData1, ttl)
	s.NoError(err)

	value, exists, err := s.cacheProvider.Get(location)
	s.NoError(err)
	s.True(exists)
	s.NotNil(value)
	s.Equal(temperature1, value.Temperature)
	s.Equal(warning1, value.Warning)

	cacheData2 := &inmemorycache.WeatherCacheData{
		Temperature: temperature2,
		Warning:     warning2,
	}

	err = s.cacheProvider.Set(location, cacheData2, ttl)
	s.NoError(err)

	value, exists, err = s.cacheProvider.Get(location)
	s.NoError(err)
	s.True(exists)
	s.NotNil(value)
	s.Equal(temperature2, value.Temperature)
	s.Equal(warning2, value.Warning)
}

func (s *InMemoryCacheTestSuite) TestMultipleLocations() {
	location1 := "London"
	temperature1 := 18.5
	warning1 := "Using only first weather API data"
	location2 := "Tokyo"
	temperature2 := 30.0
	warning2 := "Using only second weather API data"
	ttl := 5 * time.Minute

	cacheData1 := &inmemorycache.WeatherCacheData{
		Temperature: temperature1,
		Warning:     warning1,
	}

	err := s.cacheProvider.Set(location1, cacheData1, ttl)
	s.NoError(err)

	cacheData2 := &inmemorycache.WeatherCacheData{
		Temperature: temperature2,
		Warning:     warning2,
	}

	err = s.cacheProvider.Set(location2, cacheData2, ttl)
	s.NoError(err)

	value1, exists1, err1 := s.cacheProvider.Get(location1)
	s.NoError(err1)
	s.True(exists1)
	s.NotNil(value1)
	s.Equal(temperature1, value1.Temperature)
	s.Equal(warning1, value1.Warning)

	value2, exists2, err2 := s.cacheProvider.Get(location2)
	s.NoError(err2)
	s.True(exists2)
	s.NotNil(value2)
	s.Equal(temperature2, value2.Temperature)
	s.Equal(warning2, value2.Warning)
}

func (s *InMemoryCacheTestSuite) TestAutomaticCleanup() {
	location := "Sydney"
	temperature := 28.0
	ttl := 50 * time.Millisecond

	cacheData := &inmemorycache.WeatherCacheData{
		Temperature: temperature,
	}

	err := s.cacheProvider.Set(location, cacheData, ttl)
	s.NoError(err)

	value, exists, err := s.cacheProvider.Get(location)
	s.NoError(err)
	s.True(exists)
	s.NotNil(value)
	s.Equal(temperature, value.Temperature)

	time.Sleep(200 * time.Millisecond)

	value, exists, err = s.cacheProvider.Get(location)
	s.NoError(err)
	s.False(exists)
	s.Nil(value)
}

func (s *InMemoryCacheTestSuite) TestConcurrentAccess() {
	location := "Madrid"
	temperature := 35.0
	ttl := 5 * time.Minute
	iterations := 100

	cacheData := &inmemorycache.WeatherCacheData{
		Temperature: temperature,
	}

	err := s.cacheProvider.Set(location, cacheData, ttl)
	s.NoError(err)

	done := make(chan bool)
	for i := 0; i < iterations; i++ {
		go func() {
			value, exists, err := s.cacheProvider.Get(location)
			s.NoError(err)
			s.True(exists)
			s.NotNil(value)
			s.Equal(temperature, value.Temperature)
			done <- true
		}()
	}

	for i := 0; i < iterations; i++ {
		<-done
	}

	for i := 0; i < iterations; i++ {
		go func(temp float64) {
			newCacheData := &inmemorycache.WeatherCacheData{
				Temperature: temp,
			}
			err := s.cacheProvider.Set(location, newCacheData, ttl)
			s.NoError(err)
			done <- true
		}(temperature + float64(i))
	}

	for i := 0; i < iterations; i++ {
		<-done
	}

	value, exists, err := s.cacheProvider.Get(location)
	s.NoError(err)
	s.True(exists)
	s.NotNil(value)
	s.GreaterOrEqual(value.Temperature, temperature)
	s.LessOrEqual(value.Temperature, temperature+float64(iterations-1))
}

func TestInMemoryCacheTestSuite(t *testing.T) {
	suite.Run(t, new(InMemoryCacheTestSuite))
}
