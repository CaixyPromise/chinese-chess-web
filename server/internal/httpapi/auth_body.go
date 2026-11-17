package httpapi

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

// Finish receiving untrusted bytes before entering the identity gate. A context
// deadline cannot interrupt a net/http request body read: set its real socket
// deadline as well, then replace the network stream with a bounded memory body.
func readAuthBody(w http.ResponseWriter, r *http.Request) bool {
	controller := http.NewResponseController(w)
	if err := controller.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		respond(w, 503, map[string]string{"code": "UNAVAILABLE"})
		return false
	}
	defer controller.SetReadDeadline(time.Time{})
	body := http.MaxBytesReader(w, r.Body, 65536)
	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil {
		var limit *http.MaxBytesError
		var timeout net.Error
		switch {
		case errors.As(err, &limit):
			respond(w, 413, map[string]string{"code": "INVALID_REQUEST"})
		case errors.As(err, &timeout) && timeout.Timeout():
			respond(w, 408, map[string]string{"code": "TIMEOUT"})
		default:
			respond(w, 400, map[string]string{"code": "INVALID_REQUEST"})
		}
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return true
}
