package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/things-go/go-socks5"
	"golang.org/x/net/proxy"
)

const (
	addr           = "127.0.0.1:8001"
	proxyServerDNS = "127.0.0.1:9050"
	proxyServer    = "127.0.0.1:9050"
	httpServer     = "/tmp/socks5-tls.sock"
	configFilename = "config.txt"
)

var dnsServers = []string{
	"https://dns.quad9.net/dns-query",
	"https://all.dns.mullvad.net/dns-query",
	"https://security.cloudflare-dns.com/dns-query",
}

func main() {
	os.Remove(httpServer)

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

	exitDial, err := proxy.SOCKS5("tcp", proxyServer, nil, proxy.Direct)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to proxy server")
	}
	dialer := exitDial.(proxy.ContextDialer)

	dialDNS, err := proxy.SOCKS5("tcp", proxyServerDNS, nil, proxy.Direct)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to dns proxy server")
	}
	dialerDNS := dialDNS.(proxy.ContextDialer)

	go startMITMServer(dialer)

	server := socks5.NewServer(
		socks5.WithResolver(NewDNSResolver(dialerDNS)),
		socks5.WithDial(NewDialFn()),
		socks5.WithHookReplySuccess(NewHookReplySuccess()),
	)

	if err := server.ListenAndServe("tcp", addr); err != nil {
		log.Fatal().Err(err).Msg("listen server")
	}
}
