package main

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"golang.org/x/net/proxy"
)

func isWebsocketUpgrade(r *http.Request) bool {
	return r.Header.Get("Upgrade") == "websocket"
}

func handleWebSocket(ctx context.Context, dialer proxy.ContextDialer, conn *websocket.Conn, r *http.Request) error {
	switch r.URL.Scheme {
	case "https":
		r.URL.Scheme = "wss"
	case "http":
		r.URL.Scheme = "ws"
	}

	c, _, err := websocket.Dial(ctx, r.URL.String(), nil)
	if err != nil {
		return err
	}
	defer c.CloseNow()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go copyWS(ctx, cancel, conn, c)
	go copyWS(ctx, cancel, c, conn)
	<-ctx.Done()

	return nil
}

func copyWS(ctx context.Context, cancel context.CancelFunc, dst, src *websocket.Conn) {
	defer cancel()
	for {
		t, data, err := src.Read(ctx)
		if err != nil {
			return
		}
		dst.Write(ctx, t, data)
	}
}
