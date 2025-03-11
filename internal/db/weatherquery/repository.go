package weatherquery

import (
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	LogWeatherQuery(location string, service1Temp, service2Temp float64, requestCount int) error
	GetRecentWeatherQuery(location string) (*WeatherQuery, error)
}

type WeatherSQLRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &WeatherSQLRepository{db: db}
}

func (r *WeatherSQLRepository) LogWeatherQuery(location string, service1Temp, service2Temp float64, requestCount int) error {
	query := WeatherQuery{
		Location:            location,
		Service1Temperature: service1Temp,
		Service2Temperature: service2Temp,
		RequestCount:        requestCount,
		CreatedAt:           time.Now(),
	}

	return r.db.Create(&query).Error
}

func (r *WeatherSQLRepository) GetRecentWeatherQuery(location string) (*WeatherQuery, error) {
	var query WeatherQuery
	err := r.db.Where("location = ?", location).Order("created_at DESC").First(&query).Error
	if err != nil {
		return nil, err
	}
	return &query, nil
}
