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

		host, _, _ := net.SplitHostPort(addr)
		if host == blockIP.String() {
			return net.Dial(network, addr)
		}

		// mu.RLock()
		// defer mu.RLocker().Unlock()
		// if _, found := proxyMapIP[host]; found {
		// 	logger.Info().Msg("proxy dial")
		// 	return dial.Dial(network, addr)
		// }
		// if ip, found := proxyMapDomain[host]; found {
		// 	logger.Info().Msg("proxy dial")
		// 	return dial.Dial(network, ip)
		// }

		logger.Info().Msg("dial")
		return net.Dial(network, "127.0.0.1:8080")
		// return net.Dial(network, addr)
	}
}
