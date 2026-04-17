package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

func startMITMServer(dialer proxy.ContextDialer) {
	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			var body []byte
			if r.Body != nil {
				ibody, _ := io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewReader(ibody))
				body = ibody
			}

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("body", string(body)).
				Msg("peek")
			r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:136.0) Gecko/20100101 Firefox/136.0")
		},
		ModifyResponse: func(r *http.Response) error {
			if val, found := proxyList.MatchEx(r.Request.Host); found && val == "c" {
				setCORS(r.Header)
			}
			return nil
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, _, _ := net.SplitHostPort(addr)
				if isProxy(host) {
					return dialer.DialContext(ctx, network, addr)
				}
				return net.Dial(network, addr)
			},
		},
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
			defer conn.Close()

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

			var req *http.Request
			if isTLS {
				tlsConn := tls.Server(conn, &tls.Config{
					GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
						return generateCert(hello.ServerName, caKeyPair, caX509)
					},
				})
				if err := tlsConn.Handshake(); err != nil {
					log.Err(err).Msg("tls handshake")
					return
				}
				conn = tlsConn
			}

			req, err = http.ReadRequest(bufio.NewReader(conn))
			if err != nil {
				log.Err(err).Msg("read request")
				return
			}
			req.URL.Host = req.Host
			req.URL.Scheme = "http"
			if isTLS {
				req.URL.Scheme = "https"
			}

			rw := newConnResponseWriter(conn, req)
			rp.ServeHTTP(rw, req)
		}(conn)
	}
}

type peekConn struct {
	net.Conn
	r io.Reader
}

func (c *peekConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

type connResponseWriter struct {
	request     *http.Request
	conn        net.Conn
	header      http.Header
	wroteHeader bool
}

func newConnResponseWriter(conn net.Conn, req *http.Request) *connResponseWriter {
	return &connResponseWriter{conn: conn, header: http.Header{}, request: req}
}

func (w *connResponseWriter) Header() http.Header {
	return w.header
}

func (w *connResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	fmt.Fprintf(w.conn, "HTTP/1.1 %d %s\r\n", code, http.StatusText(code))
	w.header.Write(w.conn)
	fmt.Fprint(w.conn, "\r\n")
}

func (w *connResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.conn.Write(b)
}

func setCORS(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Headers", "*")
	header.Set("Access-Control-Allow-Methods", "*")
	header.Set("Access-Control-Expose-Headers", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
}
