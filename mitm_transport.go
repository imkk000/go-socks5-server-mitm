package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	utls "github.com/refraction-networking/utls"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/proxy"
)

type isTLSCtx struct{}

func isTLS(ctx context.Context) bool {
	val := ctx.Value(&isTLSCtx{})
	if val == nil {
		return false
	}
	v, ok := val.(bool)
	if !ok {
		return false
	}
	return v
}

type RoundTripper struct {
	h1 *http.Transport
	h2 *http2.Transport
}

func (r RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "https" {
		return r.h2.RoundTrip(req)
	}
	return r.h1.RoundTrip(req)
}

func NewRoundTripper(dialer proxy.ContextDialer) RoundTripper {
	fn := NewDialTLSContext(dialer)
	return RoundTripper{
		h1: &http.Transport{
			DialTLSContext: fn,
		},
		h2: &http2.Transport{
			DialTLSContext: func(ctx context.Context, network string, addr string, _ *tls.Config) (net.Conn, error) {
				return fn(ctx, network, addr)
			},
		},
	}
}

func NewDialTLSContext(dialer proxy.ContextDialer) func(ctx context.Context, network string, addr string) (net.Conn, error) {
	return func(ctx context.Context, network string, addr string) (net.Conn, error) {
		var conn net.Conn
		var err error
		host, _, _ := net.SplitHostPort(addr)
		if isProxy(host) {
			conn, err = dialer.DialContext(ctx, network, addr)
		} else {
			conn, err = net.Dial(network, addr)
		}
		if err != nil {
			return nil, err
		}
		if isTLS(ctx) {
			helloClientID := getClientHelloID(ctx)
			uconn := utls.UClient(conn, &utls.Config{
				ServerName: host,
			}, helloClientID)
			if err := uconn.Handshake(); err != nil {
				return nil, err
			}
			state := uconn.ConnectionState()
			log.Info().
				Str("host", host).
				Str("proto", state.NegotiatedProtocol).
				Uint16("cipher", state.CipherSuite).
				Str("client_id", helloClientID.Str()).
				Msg("get connection state")

			return uconn, nil
		}

		return conn, nil
	}
}
