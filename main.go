package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/things-go/go-socks5"
	"golang.org/x/net/proxy"
)

const (
	addr           = "127.0.0.1:8000"
	dnsServer      = "127.0.0.1:9059"
	proxyServer    = "127.0.0.1:9050"
	tlsServer      = "127.0.0.1:8080"
	configFilename = "config.txt"
)

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:             os.Stdout,
		FormatTimestamp: func(any) string { return "" },
	})

	config, err := readConfig(configFilename)
	if err != nil {
		log.Fatal().Err(err).Msg("read config")
	}
	for _, cfg := range config {
		switch cfg.Status {
		case "b":
			blockList[cfg.Domain] = struct{}{}
		case "p":
			proxyList[cfg.Domain] = struct{}{}
		default:
		}
	}

	dial, err := proxy.SOCKS5("tcp", proxyServer, nil, proxy.Direct)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to proxy server")
	}

	go func() {
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
			},
			ModifyResponse: func(resp *http.Response) error {
				setCORS(resp.Header)
				return nil
			},
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					// call socks5 proxy (torsocks)
					return dial.Dial(network, addr)
				},
			},
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				setCORS(w.Header())
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
	}()

	server := socks5.NewServer(
		socks5.WithResolver(new(DNSResolver)),
		socks5.WithDial(NewDialFn(dial)),
	)

	if err := server.ListenAndServe("tcp", addr); err != nil {
		log.Fatal().Err(err).Msg("listen server")
	}
}

var mu = new(sync.RWMutex)

func setCORS(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Headers", "*")
	header.Set("Access-Control-Allow-Methods", "*")
	header.Set("Access-Control-Expose-Headers", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
}
