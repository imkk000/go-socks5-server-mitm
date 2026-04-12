package main

import (
	"bytes"
	"context"
	"io"
	"net"

	"github.com/rs/zerolog/log"
	"github.com/things-go/go-socks5"
	"golang.org/x/net/proxy"
)

type (
	DialFn             = func(ctx context.Context, network, addr string) (net.Conn, error)
	HookReplySuccessFn = func(ctx context.Context, conn net.Conn, request *socks5.Request) (net.Conn, *socks5.Request, error)
)

func NewHookReplySuccess(dialer proxy.ContextDialer) HookReplySuccessFn {
	return func(ctx context.Context, conn net.Conn, req *socks5.Request) (net.Conn, *socks5.Request, error) {
		val, found := proxyMapIP.Load(req.DestAddr.IP.String())
		if !found {
			return conn, req, nil
		}
		host := val.(string)

		if !isProxy(host) {
			return conn, req, nil
		}

		val, found = dnsTLSCache.Load(host)
		if !found {
			b := make([]byte, 1)
			n, err := req.Reader.Read(b)
			if err != nil || n == 0 {
				return nil, nil, err
			}
			req.Reader = io.MultiReader(bytes.NewReader(b[:n]), req.Reader)
			isTLS := b[0] == 0x16
			val = isTLS
			dnsTLSCache.Store(host, isTLS)
		}
		isTLS := val.(bool)
		if !isTLS {
			return conn, req, nil
		}
		conn.Close()
		dialer := &net.Dialer{}
		conn, err := dialer.DialContext(ctx, req.RemoteAddr.Network(), tlsServer)
		if err != nil {
			return nil, nil, err
		}
		return conn, req, nil
	}
}

func NewDialFn(dialer proxy.ContextDialer) DialFn {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		logger := log.With().Str("addr", addr).Logger()

		host, _, _ := net.SplitHostPort(addr)
		if host == net.IPv4zero.String() || isSkipProxy(host) {
			return net.Dial(network, addr)
		}
		isProxyMode := isProxy(host)
		if isProxyMode {
			logger.Info().Msg("dial proxy")
			return dialer.DialContext(ctx, network, addr)
		}

		logger.Info().Msg("dial")
		return net.Dial(network, addr)
	}
}
