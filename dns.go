package main

import (
	"context"
	"net"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
)

type DNSResolver int

func (d *DNSResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	logger := log.With().Str("query", name).Logger()

	if isBlock(name) {
		logger.Info().Msg("blocked")
		return ctx, blockIP, nil
	}

	msg := new(dns.Msg)
	msg.SetQuestion(name+".", dns.TypeA)
	client := new(dns.Client)
	resp, _, err := client.Exchange(msg, dnsServer)
	if err != nil {
		logger.Err(err).Msg("exchanged")
		return ctx, nil, err
	}
	for _, answer := range resp.Answer {
		if a, ok := answer.(*dns.A); ok {
			if isProxy(name) {
				mu.Lock()
				proxyMapIP[a.A.String()] = name
				proxyMapDomain[name] = a.A.String()
				mu.Unlock()
				logger.Info().Msg("exchanged (proxy)")

				return ctx, a.A, nil
			}
		}
	}
	logger.Info().Msg("exchanged")

	return ctx, nil, err
}

var (
	blockIP   = net.ParseIP("0.0.0.0")
	blockList = make(map[string]struct{})
)

func isBlock(name string) bool {
	_, found := blockList[name]
	return found
}

var (
	proxyList      = make(map[string]struct{})
	proxyMapDomain = make(map[string]string)
	proxyMapIP     = make(map[string]string)
)

func isProxy(name string) bool {
	_, found := proxyList[name]
	return found
}
