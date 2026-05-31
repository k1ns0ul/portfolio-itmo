package api

type SuccessResponse struct {
	Data any `json:"data"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type PaginatedResponse struct {
	Data    any  `json:"data"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

const (
	ErrCodeNotFound     = "not_found"
	ErrCodeBadRequest   = "bad_request"
	ErrCodeUnauthorized = "unauthorized"
	ErrCodeForbidden    = "forbidden"
	ErrCodeConflict     = "conflict"
	ErrCodeInternal     = "internal"
	ErrCodeUnavailable  = "unavailable"
)
