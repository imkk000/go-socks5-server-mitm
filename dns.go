package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

type DNSResolver struct {
	client *http.Client
}

func NewDNSResolver(dial proxy.ContextDialer) *DNSResolver {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dial.DialContext(ctx, network, addr)
			},
			ForceAttemptHTTP2: true,
		},
	}
	return &DNSResolver{client}
}

func (d *DNSResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	// TODO: resolve to localhost all cases no blocked
	// using doh server int localhost
	logger := log.With().Str("query", name).Logger()

	if isBlock(name) {
		logger.Info().Msg("blocked")
		return ctx, net.IPv4zero, nil
	}

	// cache
	if val, hit := dnsMemCache.Load(name); hit {
		c := val.(DNSCache)
		if time.Until(c.Expiry).Seconds() > 0 {
			logger.Info().Msg("hit cache")
			return ctx, c.IP, nil
		}
	}

	msg := new(dns.Msg)
	msg.SetQuestion(name+".", dns.TypeA)
	wire, err := msg.Pack()
	if err != nil {
		return ctx, nil, err
	}
	httpResp, err := d.doRequest(ctx, wire)
	if err != nil {
		return ctx, nil, err
	}
	defer httpResp.Body.Close()

	content, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return ctx, nil, err
	}
	resp := new(dns.Msg)
	resp.Unpack(content)

	for _, answer := range resp.Answer {
		if a, ok := answer.(*dns.A); ok {
			logger.Info().Msg("exchanged")

			dnsMemCache.Store(name, DNSCache{
				IP:     a.A,
				Expiry: time.Now().Add(time.Duration(a.Hdr.Ttl) * time.Second),
			})
			proxyMapIP.Store(a.A.String(), name)

			return ctx, a.A, nil
		}
	}
	logger.Info().Msg("exchanged")
	return ctx, nil, nil
}

func (d *DNSResolver) doRequest(ctx context.Context, rawMsg []byte) (*http.Response, error) {
	for _, server := range dnsServers {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server, bytes.NewReader(rawMsg))
		if err != nil {
			continue
		}
		req.ContentLength = int64(len(rawMsg))
		req.Header.Set("Content-Type", "application/dns-message")
		req.Header.Set("Accept", "application/dns-message")
		resp, err := d.client.Do(req)
		if err != nil {
			continue
		}
		return resp, nil
	}

	return nil, errors.New("all upstream servers failed")
}

var (
	dnsMemCache = new(sync.Map)
	dnsTLSCache = new(sync.Map)
)

type DNSCache struct {
	IP     net.IP
	Expiry time.Time
}

var blockList = NewTrie()

func isBlock(name string) bool {
	return blockList.Match(name)
}

var (
	// TODO: evict
	proxyMapIP = new(sync.Map)
	proxyList  = NewTrie()
)

func isProxy(name string) bool {
	val, found := proxyMapIP.Load(name)
	if found {
		name = val.(string)
	}
	return proxyList.Match(name)
}

var skipProxy = NewTrie()

func isSkipProxy(name string) bool {
	return skipProxy.Match(name)
}
