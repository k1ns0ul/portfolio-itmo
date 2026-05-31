package api

import (
	"bytes"
	"net/http"
)

type recorder struct {
	headers http.Header
	body    *bytes.Buffer
	status  int
}

func newRecorder() *recorder {
	return &recorder{headers: make(http.Header), body: new(bytes.Buffer), status: http.StatusOK}
}

func (r *recorder) Header() http.Header {
	return r.headers
}

func (r *recorder) Write(p []byte) (int, error) {
	return r.body.Write(p)
}

func (r *recorder) WriteHeader(status int) {
	r.status = status
}
