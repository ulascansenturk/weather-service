package handlers

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"net/http"
)

func respondWithError(w http.ResponseWriter, code int, message string) {
	errorCode := "INTERNAL_ERROR"
	title := "Internal Server Error"

	switch code {
	case http.StatusBadRequest:
		errorCode = "BAD_REQUEST"
		title = "Bad Request"
	case http.StatusNotFound:
		errorCode = "NOT_FOUND"
		title = "Not Found"
	case http.StatusMethodNotAllowed:
		errorCode = "METHOD_NOT_ALLOWED"
		title = "Method Not Allowed"
	}

	respondWithJSON(w, code, ErrorResponse{
		Errors: []Error{
			{
				Code:   errorCode,
				Detail: message,
				Status: code,
				Title:  title,
			},
		},
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Error().Err(err).Msg("failed to encode response")
	}
}
