package config

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"time"
)

type Config struct {
	ServiceName   string
	ServerAddress string

	DBName     string
	DBPassword string
	DBUser     string
	DBPort     string
	DBHost     string

	Env         string
	LogLevel    string
	HTTPTimeout int32

	WeatherApiAPIKey   string
	WeatherStackAPIKey string

	MaxQueueSize   int
	MaxWaitTime    time.Duration
	CacheTTL       time.Duration
	FailedCacheTTL time.Duration
}

func LoadConfig() (*Config, error) {
	v := viper.New()

	v.SetDefault("SERVICE_NAME", "weather-service")

	v.SetDefault("SERVER_ADDRESS", "0.0.0.0:3000")
	v.SetDefault("DATABASE_PORT", "5432")
	v.SetDefault("HTTP_TIMEOUT", 175)
	v.SetDefault("MAX_QUEUE_SIZE", 10)
	v.SetDefault("MAX_WAIT_TIME", 5*time.Second)
	v.SetDefault("CACHE_TTL", 5*time.Minute)
	v.SetDefault("FAILED_CACHE_TTL", 2*time.Minute)

	v.AutomaticEnv()

	v.SetConfigName(".env")
	v.SetConfigType("env")
	v.AddConfigPath(".")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Warn().Msg("No .env file found, using environment variables only")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		log.Info().Str("file", v.ConfigFileUsed()).Msg("Config file loaded")
	}

	config := &Config{
		ServiceName:        v.GetString("SERVICE_NAME"),
		ServerAddress:      v.GetString("SERVER_ADDRESS"),
		DBName:             v.GetString("DATABASE_NAME"),
		DBPassword:         v.GetString("DATABASE_PASSWORD"),
		DBUser:             v.GetString("DATABASE_USER"),
		DBPort:             v.GetString("DATABASE_PORT"),
		DBHost:             v.GetString("DATABASE_HOST"),
		Env:                v.GetString("ENV"),
		LogLevel:           v.GetString("LOG_LEVEL"),
		HTTPTimeout:        v.GetInt32("HTTP_TIMEOUT"),
		WeatherApiAPIKey:   v.GetString("WEATHER_API_API_KEY"),
		WeatherStackAPIKey: v.GetString("WEATHER_STACK_API_KEY"),
		MaxQueueSize:       v.GetInt("MAX_QUEUE_SIZE"),
		MaxWaitTime:        v.GetDuration("MAX_WAIT_TIME"),
		CacheTTL:           v.GetDuration("CACHE_TTL"),
		FailedCacheTTL:     v.GetDuration("FAILED_CACHE_TTL"),
	}

	return config, nil
}

func (c *Config) HTTPTimeoutDuration() time.Duration {
	return time.Duration(c.HTTPTimeout) * time.Second
}
