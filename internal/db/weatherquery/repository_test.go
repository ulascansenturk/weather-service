package weatherquery_test

import (
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testing"
	"time"
	"ulascansenturk/weather-service/internal/db/weatherquery"
)

type WeatherRepositorySuite struct {
	suite.Suite
	DB   *gorm.DB
	mock sqlmock.Sqlmock
	repo weatherquery.Repository
}

func (s *WeatherRepositorySuite) SetupSuite() {
	var err error

	var db *sql.DB
	db, s.mock, err = sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	s.Require().NoError(err)

	dialector := postgres.New(postgres.Config{
		DSN:                  "sqlmock_db_0",
		DriverName:           "postgres",
		Conn:                 db,
		PreferSimpleProtocol: true,
	})

	s.DB, err = gorm.Open(dialector, &gorm.Config{})
	s.Require().NoError(err)

	s.repo = weatherquery.NewRepository(s.DB)
}

func (s *WeatherRepositorySuite) TearDownTest() {
	s.Require().NoError(s.mock.ExpectationsWereMet())
}

func (s *WeatherRepositorySuite) TestLogWeatherQuery() {
	s.Run("Successfully logs a weather query", func() {
		location := "Istanbul"
		service1Temp := 22.5
		service2Temp := 23.5
		requestCount := 3

		s.mock.ExpectBegin()
		s.mock.ExpectQuery(`INSERT INTO "weather_queries"`).
			WithArgs(
				location,
				service1Temp,
				service2Temp,
				requestCount,
				sqlmock.AnyArg(),
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
		s.mock.ExpectCommit()

		err := s.repo.LogWeatherQuery(location, service1Temp, service2Temp, requestCount)

		s.Require().NoError(err)
	})

	s.Run("Returns error when database operation fails", func() {
		location := "Paris"
		service1Temp := 18.0
		service2Temp := 19.0
		requestCount := 5
		dbError := errors.New("database error")

		s.mock.ExpectBegin()
		s.mock.ExpectQuery(`INSERT INTO "weather_queries"`).
			WithArgs(
				location,
				service1Temp,
				service2Temp,
				requestCount,
				sqlmock.AnyArg(),
			).
			WillReturnError(dbError)
		s.mock.ExpectRollback()

		err := s.repo.LogWeatherQuery(location, service1Temp, service2Temp, requestCount)

		s.Require().Error(err)
		s.Require().Equal("database error", err.Error())
	})
}

func (s *WeatherRepositorySuite) TestGetRecentWeatherQuery() {
	s.Run("Successfully retrieves the most recent weather query", func() {
		location := "London"
		service1Temp := 10.0
		service2Temp := 11.0
		requestCount := 2
		createdAt := time.Now()

		queryRegex := `SELECT \* FROM "weather_queries" WHERE location = \$1 ORDER BY created_at DESC,"weather_queries"."id" LIMIT \$2`

		rows := sqlmock.NewRows([]string{
			"id", "created_at", "updated_at", "deleted_at",
			"location", "service_1_temperature", "service_2_temperature", "request_count",
		}).AddRow(
			1, createdAt, createdAt, nil,
			location, service1Temp, service2Temp, requestCount,
		)

		s.mock.ExpectQuery(queryRegex).
			WithArgs(location, 1).
			WillReturnRows(rows)

		result, err := s.repo.GetRecentWeatherQuery(location)

		s.Require().NoError(err)
		s.Require().NotNil(result)
		s.Require().Equal(location, result.Location)
		s.Require().Equal(service1Temp, result.Service1Temperature)
		s.Require().Equal(service2Temp, result.Service2Temperature)
		s.Require().Equal(requestCount, result.RequestCount)
	})

	s.Run("Returns error when no record found", func() {
		location := "Tokyo"
		gormError := gorm.ErrRecordNotFound

		queryRegex := `SELECT \* FROM "weather_queries" WHERE location = \$1 ORDER BY created_at DESC,"weather_queries"."id" LIMIT \$2`

		s.mock.ExpectQuery(queryRegex).
			WithArgs(location, 1).
			WillReturnError(gormError)

		result, err := s.repo.GetRecentWeatherQuery(location)

		s.Require().Error(err)
		s.Require().Equal("record not found", err.Error())
		s.Require().Nil(result)
	})

	s.Run("Returns error when database query fails", func() {
		location := "Berlin"
		dbError := errors.New("connection error")

		queryRegex := `SELECT \* FROM "weather_queries" WHERE location = \$1 ORDER BY created_at DESC,"weather_queries"."id" LIMIT \$2`

		s.mock.ExpectQuery(queryRegex).
			WithArgs(location, 1).
			WillReturnError(dbError)

		result, err := s.repo.GetRecentWeatherQuery(location)

		s.Require().Error(err)
		s.Require().Equal("connection error", err.Error())
		s.Require().Nil(result)
	})
}

func TestWeatherRepositorySuite(t *testing.T) {
	suite.Run(t, new(WeatherRepositorySuite))
}
