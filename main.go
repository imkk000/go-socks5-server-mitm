package main

import (
	"net/http"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/things-go/go-socks5"
	"golang.org/x/net/proxy"
)

const (
	addr           = "127.0.0.1:8000"
	proxyServer    = "127.0.0.1:9050"
	tlsServer      = "127.0.0.1:54909"
	configFilename = "config.txt"
)

var dnsServers = []string{
	"https://dns.quad9.net/dns-query",
	"https://all.dns.mullvad.net/dns-query",
	"https://security.cloudflare-dns.com/dns-query",
}

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
		// TODO: case "c": custom dns
		switch cfg.Action {
		case "s":
			skipProxy.Insert(cfg.Domain)
		case "b":
			blockList.Insert(cfg.Domain)
		case "p":
			proxyList.Insert(cfg.Domain, cfg.Extra)
		default:
		}
	}

	dial, err := proxy.SOCKS5("tcp", proxyServer, nil, proxy.Direct)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to proxy server")
	}
	dialer := dial.(proxy.ContextDialer)
	go startMITMServer(dialer)

	server := socks5.NewServer(
		socks5.WithResolver(NewDNSResolver(dialer)),
		socks5.WithDial(NewDialFn(dialer)),
		socks5.WithHookReplySuccess(NewHookReplySuccess(dialer)),
	)

	if err := server.ListenAndServe("tcp", addr); err != nil {
		log.Fatal().Err(err).Msg("listen server")
	}
}

func setCORS(header http.Header) {
	header.Set("Access-Control-Allow-Origin", "*")
	header.Set("Access-Control-Allow-Headers", "*")
	header.Set("Access-Control-Allow-Methods", "*")
	header.Set("Access-Control-Expose-Headers", "*")
	header.Set("Access-Control-Allow-Credentials", "true")
}
