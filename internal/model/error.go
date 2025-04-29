package model

type APIError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
}