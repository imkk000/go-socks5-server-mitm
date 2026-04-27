package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	glog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/coder/websocket"
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

type browserTypeCtx struct{}

type BrowserType int

const (
	BrowserUnknown BrowserType = iota
	BrowserTypeFirefox
	BrowserTypeChrome
)

func (t BrowserType) String() string {
	switch t {
	case BrowserTypeFirefox:
		return "Firefox"
	case BrowserTypeChrome:
		return "Chrome"
	}
	return "Unknown"
}

func patchUA(ctx context.Context, r *http.Request) {
	browserType := getBrowserType(ctx)
	if browserType == BrowserTypeFirefox {
		r.Header.Set(headerUserAgent, "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:136.0) Gecko/20100101 Firefox/136.0")
		return
	}
	for k := range r.Header {
		if strings.HasPrefix(k, "Sec-Ch-Ua") {
			r.Header.Del(k)
		}
	}
	r.Header.Set(headerUserAgent, "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36")
}

func getClientHelloID(ctx context.Context) utls.ClientHelloID {
	browserType := getBrowserType(ctx)
	if browserType == BrowserTypeFirefox {
		return utls.HelloFirefox_Auto
	}
	return utls.HelloChrome_Auto
}

func getBrowserType(ctx context.Context) BrowserType {
	val := ctx.Value(&browserTypeCtx{})
	if val == nil {
		return BrowserUnknown
	}
	t, ok := val.(BrowserType)
	if !ok {
		return BrowserUnknown
	}
	return t
}

func detectBrowserType(r *http.Request) BrowserType {
	ua := r.Header.Get(headerUserAgent)
	if ua == "" {
		return BrowserUnknown
	}
	if strings.Contains(ua, "Firefox") {
		return BrowserTypeFirefox
	}
	if strings.Contains(ua, "Chrome") {
		return BrowserTypeChrome
	}
	return BrowserUnknown
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
	buf         *bufio.ReadWriter
}

func newConnResponseWriter(conn net.Conn, req *http.Request) *connResponseWriter {
	return &connResponseWriter{
		conn:    conn,
		header:  http.Header{},
		request: req,
		buf:     bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}
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

func (w *connResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.buf, nil
}

func setCORS(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Headers", "*")
	header.Set("Access-Control-Allow-Methods", "*")
	header.Set("Access-Control-Expose-Headers", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
}
