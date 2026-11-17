package httpapi

import (
	"context"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"net/http"
	"time"
)

func (a *API) socket(w http.ResponseWriter, r *http.Request) {
	r, release, closeSocket, ok := a.socketIdentity(w, r)
	if !ok {
		return
	}
	defer release()
	defer closeSocket()
	id := requireOwner(w, r)
	if id == "" {
		return
	}
	initial, updates, unwatch, err := a.games.Watch(id, r.PathValue("id"))
	if err != nil {
		a.fail(w, err)
		return
	}
	defer unwatch()
	release()
	stream(w, r, a.games.Context(), initial, updates)
}

func stream[T any](w http.ResponseWriter, r *http.Request, parent context.Context, initial T, updates <-chan T) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(1024)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	stop := context.AfterFunc(parent, cancel)
	defer stop()
	ctx = conn.CloseRead(ctx)
	write := func(v any) error {
		deadline, done := context.WithTimeout(ctx, 3*time.Second)
		defer done()
		return wsjson.Write(deadline, conn, v)
	}
	if write(initial) != nil {
		return
	}
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-updates:
			if write(v) != nil {
				return
			}
		case <-heartbeat.C:
			ping, done := context.WithTimeout(ctx, 3*time.Second)
			err = conn.Ping(ping)
			done()
			if err != nil {
				return
			}
		}
	}
}
