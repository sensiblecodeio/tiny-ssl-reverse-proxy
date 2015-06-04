package main

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type LoggingMiddleware struct {
	http.Handler
}

type ResponseRecorder struct {
	ResponseWriter http.ResponseWriter
	response       int
	*WriteCounter
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{w, 0, &WriteCounter{w, 0}}
}

func (r *ResponseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

func (r *ResponseRecorder) WriteHeader(n int) {
	r.ResponseWriter.WriteHeader(n)
	r.response = n
}

type WriteCounter struct {
	io.Writer
	nBytes int
}

func (r *WriteCounter) Write(bs []byte) (n int, err error) {
	if r.Writer != nil {
		n, err = r.Writer.Write(bs)
	} else {
		n = len(bs)
	}
	r.nBytes += n
	return n, err
}

func (x *LoggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	recorder := NewResponseRecorder(w)

	uploaded := &WriteCounter{Writer: ioutil.Discard}
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.TeeReader(r.Body, uploaded), r.Body}

	start := time.Now()
	x.Handler.ServeHTTP(recorder, r)
	duration := time.Since(start)

	log.Printf("%21v %3d %10d %10d %7.1fms %4v %v%-30v %v",
		r.RemoteAddr,
		recorder.response,
		uploaded.nBytes,
		recorder.nBytes,
		duration.Seconds()*1000,
		r.Method,
		r.URL.Host,
		r.URL.EscapedPath(),
		r.Header.Get("User-Agent"))
}
