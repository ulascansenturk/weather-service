CREATE TABLE IF NOT EXISTS weather_queries (
                                               id SERIAL PRIMARY KEY,
                                               location TEXT NOT NULL,
                                               service_1_temperature NUMERIC(5,2) NOT NULL,
    service_2_temperature NUMERIC(5,2) NOT NULL,
    request_count INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
    );

CREATE INDEX idx_weather_queries_location ON weather_queries (location);

CREATE INDEX idx_weather_queries_created_at ON weather_queries (created_at);

CREATE INDEX idx_weather_queries_location_created_at ON weather_queries (location, created_at);

COMMENT ON TABLE weather_queries IS 'Stores weather data fetched from external weather services';
