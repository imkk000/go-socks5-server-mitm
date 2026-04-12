package main

import (
	"context"
	"net"

	"github.com/rs/zerolog/log"
	"github.com/things-go/go-socks5"
)

type (
	DialFn             = func(ctx context.Context, network, addr string) (net.Conn, error)
	HookReplySuccessFn = func(ctx context.Context, conn net.Conn, request *socks5.Request) (net.Conn, *socks5.Request, error)
)

func NewHookReplySuccess() HookReplySuccessFn {
	return func(_ context.Context, conn net.Conn, req *socks5.Request) (net.Conn, *socks5.Request, error) {
		host, _, _ := net.SplitHostPort(req.Request.DstAddr.String())
		val, found := proxyMapIP.Load(host)
		if found {
			host = val.(string)
		}
		if host == net.IPv4zero.String() || isSkipProxy(host) {
			return conn, req, nil
		}

		return conn, req, nil
	}
}

func NewDialFn() DialFn {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		logger := log.With().Str("addr", addr).Logger()

		host, _, _ := net.SplitHostPort(addr)
		val, found := proxyMapIP.Load(host)
		if found {
			host = val.(string)
		}
		if host == net.IPv4zero.String() || isSkipProxy(host) {
			return net.Dial(network, addr)
		}

		logger.Info().Msg("dial")
		return net.Dial("unix", httpServer)
	}
}
