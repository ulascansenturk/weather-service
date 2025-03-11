package main

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"ulascansenturk/weather-service/config"
	"ulascansenturk/weather-service/internal/api/v1/handlers"
	"ulascansenturk/weather-service/internal/db/weatherquery"
	"ulascansenturk/weather-service/internal/inmemorycache"
	"ulascansenturk/weather-service/internal/providers"
	"ulascansenturk/weather-service/internal/service"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	logLevel, err := zerolog.ParseLevel(conf.LogLevel)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	logger := zerolog.New(os.Stdout).
		Level(logLevel).
		With().
		Str("service_name", conf.ServiceName).
		Timestamp().
		Logger()

	ctx, mainCtxStop := context.WithCancel(context.Background())

	db, dbErr := initializeDatabase(conf)
	if dbErr != nil {
		logger.Fatal().Err(err).Msg("failed to initialize database")

	}

	weatherRepo := weatherquery.NewRepository(db)

	cacheProvider := inmemorycache.NewInMemoryCacheProvider(time.Duration(time.Second * 60))

	weatherAPIService := providers.NewWeatherAPIService(conf.WeatherApiAPIKey, conf.WeatherStackAPIKey)

	aggregator := service.NewWeatherRequestAggregator(
		weatherAPIService,
		cacheProvider,
		weatherRepo,
		conf.MaxQueueSize,
		conf.MaxWaitTime,
		conf.CacheTTL,
		conf.FailedCacheTTL,
	)
	weatherService := service.NewWeatherService(aggregator)

	handler := handlers.NewWeatherHandler(weatherService, conf.HTTPTimeoutDuration())

	httpServer := &http.Server{
		Addr:              conf.ServerAddress,
		Handler:           handler,
		ReadHeaderTimeout: conf.HTTPTimeoutDuration(),
	}

	handleSignals(ctx, mainCtxStop, func() {
		shutdownErr := httpServer.Shutdown(ctx)
		if shutdownErr != nil {
			log.Fatal().Err(shutdownErr).Msg("server shutdown failed")
		}
	})

	log.Info().Msgf("started server on %s", conf.ServerAddress)

	serverErr := httpServer.ListenAndServe()
	if serverErr != nil {
		log.Err(serverErr).Msg("server stopped")
	}
	<-ctx.Done()
}

func initializeDatabase(config *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&weatherquery.WeatherQuery{}); err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(3 * time.Minute)

	return db, nil
}

func handleSignals(ctx context.Context, cancelCtx context.CancelFunc, callback func()) {
	sig := make(chan os.Signal, 1)

	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	const shutdownDuration = 30 * time.Second

	go func() {
		<-sig

		shutdownCtx, cancel := context.WithTimeout(ctx, shutdownDuration)

		go func() {
			<-shutdownCtx.Done()

			if shutdownCtx.Err() == context.DeadlineExceeded {
				panic("graceful shutdown timed out.. forcing exit.")
			}
		}()

		callback()

		cancel()
		cancelCtx()
	}()
}
