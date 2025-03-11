package weatherquery

import (
	"time"
)

type WeatherQuery struct {
	ID                  uint      `json:"id" gorm:"primaryKey"`
	Location            string    `json:"location" gorm:"index:idx_location;index:idx_location_created_at"`
	Service1Temperature float64   `json:"service_1_temperature" gorm:"column:service_1_temperature"`
	Service2Temperature float64   `json:"service_2_temperature" gorm:"column:service_2_temperature"`
	RequestCount        int       `json:"request_count" gorm:"column:request_count"`
	CreatedAt           time.Time `json:"created_at" gorm:"index:idx_created_at;index:idx_location_created_at"`
}

func (WeatherQuery) TableName() string {
	return "weather_queries"
}
