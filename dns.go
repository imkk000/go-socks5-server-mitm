package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/proxy"
)

type DNSResolver struct {
	client *http.Client
}

func NewDNSResolver(dial proxy.Dialer) *DNSResolver {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dial.Dial(network, addr)
			},
			ForceAttemptHTTP2: true,
		},
	}
	return &DNSResolver{client}
}

func (d *DNSResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	logger := log.With().Str("query", name).Logger()

	if isBlock(name) {
		logger.Info().Msg("blocked")
		return ctx, blockIP, nil
	}

	// cache
	dnsMemMutex.RLock()
	if c, hit := dnsMemCache[name]; hit {
		if time.Until(c.Expiry).Seconds() > 0 {
			defer dnsMemMutex.RUnlock()
			logger.Info().Msg("hit cache")
			return ctx, c.IP, nil
		}
	}
	dnsMemMutex.RUnlock()

	msg := new(dns.Msg)
	msg.SetQuestion(name+".", dns.TypeA)
	wire, err := msg.Pack()
	if err != nil {
		return ctx, nil, err
	}
	req, err := http.NewRequest(http.MethodPost, dnsServer, bytes.NewReader(wire))
	if err != nil {
		return ctx, nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	httpResp, err := d.client.Do(req)
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

			dnsMemMutex.Lock()
			dnsMemCache[name] = DNSCache{
				IP:     a.A,
				Expiry: time.Now().Add(time.Duration(a.Hdr.Ttl) * time.Second),
			}
			dnsMemMutex.Unlock()

			if isProxy(name) {
				proxyMutex.Lock()
				proxyMapIP[a.A.String()] = name
				proxyMutex.Unlock()
			}

			return ctx, a.A, nil
		}
	}
	logger.Info().Msg("exchanged")
	return ctx, nil, nil
}

var (
	dnsMemMutex = new(sync.RWMutex)
	dnsMemCache = make(map[string]DNSCache)
)

type DNSCache struct {
	IP     net.IP
	Expiry time.Time
}

var (
	blockIP   = net.ParseIP("0.0.0.0")
	blockList = make(map[string]struct{})
)

func isBlock(name string) bool {
	if _, found := blockList[name]; found {
		return true
	}
	for domain := range blockList {
		if strings.HasSuffix(name, domain) {
			return true
		}
	}
	return false
}

var (
	proxyMutex = new(sync.RWMutex)
	proxyList  = make(map[string]struct{})
	proxyMapIP = make(map[string]string)
)

func isProxy(name string) bool {
	proxyMutex.RLock()
	// map ip into hostname
	host, found := proxyMapIP[name]
	if found {
		name = host
	}
	proxyMutex.RUnlock()

	if _, found := proxyList[name]; found {
		return true
	}
	for domain := range proxyList {
		if strings.HasSuffix(name, domain) {
			return true
		}
	}
	return false
}
