package main

import (
	"context"
	"net"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

type DialFn = func(ctx context.Context, network, addr string) (net.Conn, error)

func NewDialFn(dial proxy.Dialer) DialFn {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		logger := log.With().Str("addr", addr).Logger()

		host, port, _ := net.SplitHostPort(addr)
		if host == blockIP.String() {
			return net.Dial(network, addr)
		}
		if isProxy(host) {
			if port == "443" {
				logger.Info().Msg("dial proxy (tls)")
				return net.Dial(network, tlsServer)
			}
			logger.Info().Msg("dial proxy")
			return dial.Dial(network, addr)
		}

		logger.Info().Msg("dial")
		return net.Dial(network, addr)
	}
}
