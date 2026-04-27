package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	glog "log"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/coder/websocket"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

const headerUserAgent = "User-Agent"

func startMITMServer(dialer proxy.ContextDialer) {
	rt := NewRoundTripper(dialer)
	rp := &httputil.ReverseProxy{
		ErrorLog: glog.New(io.Discard, "", 0),
		Director: func(r *http.Request) {
			log.Info().
				Str("scheme", r.URL.Scheme).
				Str("host", r.Host).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("browser", getBrowserType(r.Context()).String()).
				Msg("peek")
		},
		ModifyResponse: func(r *http.Response) error {
			if val, found := proxyList.MatchEx(r.Request.Host); found && val == "c" {
				setCORS(r.Header)
			}
			return nil
		},
		Transport: rt,
	}

	listener, err := net.Listen("unix", httpServer)
	if err != nil {
		log.Fatal().Err(err).Msg("listen unux server")
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Err(err).Msg("accept")
			continue
		}

		go func(conn net.Conn) {
			defer func() {
				_ = conn.Close()
			}()

			// peek https
			b := make([]byte, 1)
			n, err := conn.Read(b)
			if err != nil || n == 0 {
				log.Err(err).Msg("peek tls")
				return
			}
			isTLS := b[0] == 0x16
			conn = &peekConn{
				Conn: conn,
				r:    io.MultiReader(bytes.NewReader(b), conn),
			}

			var r *http.Request
			if isTLS {
				tlsConn := tls.Server(conn, &tls.Config{
					GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
						return getCert(hello.ServerName, caKeyPair, caX509)
					},
				})
				if err := tlsConn.Handshake(); err != nil {
					log.Err(err).Msg("tls handshake")
					return
				}
				conn = tlsConn
			}

			r, err = http.ReadRequest(bufio.NewReader(conn))
			if err != nil {
				log.Err(err).Msg("read request")
				return
			}

			if r.Method == http.MethodConnect {
				log.Info().Msg("detect method CONNECT")
			}

			r.URL.Host = r.Host
			r.URL.Scheme = "http"
			if isTLS {
				r.URL.Scheme = "https"
			}
			ctx := context.WithValue(r.Context(), &isTLSCtx{}, isTLS)
			ctx = context.WithValue(ctx, &browserTypeCtx{}, detectBrowserType(r))
			patchUA(ctx, r)
			r = r.WithContext(ctx)
			rw := newConnResponseWriter(conn, r)
			if isWebsocketUpgrade(r) {
				conn, err := websocket.Accept(rw, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
				if err != nil {
					log.Error().Err(err).Msg("accept websocket")
					return
				}
				if err := handleWebSocket(ctx, dialer, conn, r); err != nil {
					log.Error().Err(err).Msg("upgrade websocket")
				}
				return
			}
			rp.ServeHTTP(rw, r)
		}(conn)
	}
}

func setCORS(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Headers", "*")
	header.Set("Access-Control-Allow-Methods", "*")
	header.Set("Access-Control-Expose-Headers", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
}
