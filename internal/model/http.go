package model

import (
	"net/http"
	"time"
)

type Request struct {
	Method string;
	URL string;
	Headers http.Header;
	Body []byte
}

type Response struct {
	Request *Request;
	Headers http.Header;
	Body []byte
	StatusCode int;
	FetchedAt time.Time
}

