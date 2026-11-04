package diagnostics

import (
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// WrapTransport returns a RoundTripper that records every engine request. The
// response body is preserved exactly; accounting is completed when it reaches
// EOF or is closed, so bytesOut reflects the bytes actually consumed. A nil
// base uses http.DefaultTransport and a nil recorder returns base unchanged.
func WrapTransport(base http.RoundTripper, r *Recorder) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if r == nil {
		return base
	}
	return &recordingTransport{base: base, recorder: r}
}

type recordingTransport struct {
	base     http.RoundTripper
	recorder *Recorder
}

func (t *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	started := time.Now()
	method := request.Method
	path := request.URL.Path
	if path == "" {
		path = "/"
	}
	var requestBytes int64
	if request.ContentLength > 0 {
		requestBytes = request.ContentLength
	}

	response, err := t.base.RoundTrip(request)
	if err != nil {
		t.recorder.ObserveEngineRequest(method, path, 0, requestBytes, 0, time.Since(started))
		return response, err
	}
	if response.Body == nil {
		t.recorder.ObserveEngineRequest(method, path, response.StatusCode, requestBytes, 0, time.Since(started))
		return response, nil
	}

	response.Body = &recordingBody{
		body:         response.Body,
		recorder:     t.recorder,
		method:       method,
		path:         path,
		status:       response.StatusCode,
		requestBytes: requestBytes,
		started:      started,
	}
	return response, nil
}

type recordingBody struct {
	body         io.ReadCloser
	recorder     *Recorder
	method       string
	path         string
	status       int
	requestBytes int64
	started      time.Time

	read   atomic.Int64
	finish sync.Once
}

func (b *recordingBody) Read(p []byte) (int, error) {
	n, err := b.body.Read(p)
	b.read.Add(int64(n))
	if err == io.EOF {
		b.record()
	}
	return n, err
}

func (b *recordingBody) Close() error {
	err := b.body.Close()
	b.record()
	return err
}

func (b *recordingBody) record() {
	b.finish.Do(func() {
		b.recorder.ObserveEngineRequest(
			b.method,
			b.path,
			b.status,
			b.requestBytes,
			b.read.Load(),
			time.Since(b.started),
		)
	})
}
