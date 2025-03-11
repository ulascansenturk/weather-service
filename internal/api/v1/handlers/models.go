package handlers

type WeatherResponse struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Warning     string  `json:"warning,omitempty"`
}

type Error struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
	Title  string `json:"title"`
}

type ErrorResponse struct {
	Errors []Error `json:"errors"`
}
