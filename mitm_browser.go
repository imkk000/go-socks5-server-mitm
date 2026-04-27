package main

import (
	"context"
	"net/http"
	"strings"

	utls "github.com/refraction-networking/utls"
)

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

func getClientHelloID(ctx context.Context) utls.ClientHelloID {
	browserType := getBrowserType(ctx)
	if browserType == BrowserTypeFirefox {
		return utls.HelloFirefox_Auto
	}
	return utls.HelloChrome_Auto
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
