package api

type SuccessResponse struct {
	Data any `json:"data"`
}

type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type PaginatedResponse struct {
	Data    any    `json:"data"`
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}

const (
	ErrCodeNotFound    = "not_found"
	ErrCodeBadRequest  = "bad_request"
	ErrCodeRateLimited = "rate_limited"
	ErrCodeUnauthorized = "unauthorized"
	ErrCodeInternal    = "internal"
)
