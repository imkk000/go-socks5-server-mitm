package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

func startMITMServer(dial proxy.ContextDialer) {
	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			domain := hello.ServerName
			return getCert(domain, caKeyPair, caX509), nil
		},
	}

	proxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Scheme = "https"
			if r.URL.Host == "" {
				r.URL.Host = r.Host
			}
			r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:136.0) Gecko/20100101 Firefox/136.0")
		},
		ModifyResponse: func(resp *http.Response) error {
			if val, found := proxyList.MatchEx(resp.Request.Host); found && val == "c" {
				setCORS(resp.Header)
			}

			return nil
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				// call socks5 proxy (torsocks)
				// return net.Dial(network, addr)
				return dial.DialContext(ctx, network, addr)
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			if val, found := proxyList.MatchEx(r.Host); found && val == "c" {
				setCORS(w.Header())
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}
		proxy.ServeHTTP(w, r)
	})

	listener, err := tls.Listen("tcp", tlsServer, tlsConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("listen tls server")
	}
	if err := http.Serve(listener, mux); err != nil {
		log.Fatal().Err(err).Msg("start https proxy")
	}
}
