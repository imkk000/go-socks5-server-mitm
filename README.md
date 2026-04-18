# Go SOCKS5 MITM Proxy Server

A SOCKS5 proxy written in Go that performs full HTTPS inspection (MITM) — similar to how Cloudflare operates in proxy mode. Every HTTPS request from the browser is transparently decrypted, logged, and optionally modified before being forwarded upstream.

## Why

I wanted complete visibility into browser traffic: inspect HTTPS requests, inject CORS headers, block trackers, and route specific domains through Tor — all without relying on external tools. The result is a self-contained SOCKS5 proxy that acts as a local MITM with per-domain routing rules.

## How It Works

```
Browser (HPKP removed)
    │
    ▼
SOCKS5 Proxy :8001
    ├── DNS resolver (DoH via Tor, with in-memory TTL cache)
    ├── Block list   → returns 0.0.0.0 (drops connection)
    ├── Skip list    → direct dial (bypass MITM)
    └── MITM path   → dial Unix socket /tmp/socks5-tls.sock
                            │
                            ▼
                    MITM HTTP/HTTPS Server
                    ├── Peek first byte (0x16 = TLS ClientHello)
                    ├── TLS: generate cert on-the-fly for SNI domain
                    │         (signed by local root CA via mkcert fork)
                    └── Reverse proxy to real upstream
                              ├── Direct or via Tor SOCKS5
                              ├── Log method / path / body
                              └── Optionally inject CORS headers
```

After the SOCKS5 handshake succeeds, the custom `go-socks5` fork hooks into the reply-success state and redirects all non-blocked, non-skipped connections to the local MITM server over a Unix socket instead of dialing the real destination directly.

## Companion Projects

| Project | Role |
|---|---|
| [imkk000/go-socks5](https://github.com/imkk000/go-socks5) | Fork of `things-go/go-socks5` — adds `WithHookReplySuccess` to inject a handler after the SOCKS5 reply is sent to the client, enabling Unix socket redirection |
| [imkk000/mkcert](https://github.com/imkk000/mkcert) | Fork of `FiloSottile/mkcert` — customised root CA certificate details for the local CA that signs per-domain generated certs |
| [imkk000/librewolf-own-patch](https://github.com/imkk000/librewolf-own-patch) | Patches LibreWolf/Firefox to strip HPKP (HTTP Public Key Pinning), allowing the self-signed per-domain certs to be trusted without pin errors |

## Per-Domain Routing

Rules are loaded from `config.txt` at startup. Each line is:

```
<action> <domain> [extra]
```

| Action | Meaning |
|---|---|
| `b` | **Block** — DNS resolves to `0.0.0.0`, connection is dropped |
| `p` | **Proxy** — traffic for this domain is forwarded via Tor SOCKS5 upstream |
| `s` | **Skip** — bypass MITM, dial the destination directly |

Domain matching uses a trie, so wildcard-style suffix matching works (e.g. `example.com` matches `sub.example.com`).

Example `config.txt`:

```
b ads.example.com
b tracker.example.com
p restricted-site.example.com
s trusted-internal.example.com
```

## DNS Resolution

DNS is resolved inside the proxy using **DNS-over-HTTPS (DoH)** tunnelled through Tor SOCKS5, so DNS queries never leak to the local network. Responses are cached in memory using the TTL from the DNS answer. Upstream DoH servers (in order of preference):

- `https://dns.quad9.net/dns-query`
- `https://all.dns.mullvad.net/dns-query`
- `https://security.cloudflare-dns.com/dns-query`

## Certificate Generation

When an HTTPS connection arrives at the MITM server, the TLS `ClientHello` SNI is used to generate a certificate for that exact domain, signed by the local root CA. Generated certs are cached in memory to avoid regenerating on repeated visits. The browser trusts these certs because the root CA is installed as a trusted authority and HPKP is patched out.

## Setup

**1. Generate a root CA with the mkcert fork**

```sh
mkcert -install
```

Place the generated CA key and cert under `cert/`.

**2. Patch the browser**

Apply [librewolf-own-patch](https://github.com/imkk000/librewolf-own-patch) to remove HPKP enforcement and install the root CA.

**3. Configure domain rules**

Create `config.txt` and add your rules.

**4. Start Tor** (or any SOCKS5 upstream on `127.0.0.1:9050`)

**5. Run the proxy**

```sh
go run .
```

The SOCKS5 proxy listens on `127.0.0.1:8001`. Point your browser's SOCKS5 proxy setting there.

## Dependencies

- [things-go/go-socks5](https://github.com/things-go/go-socks5) (replaced by the fork above via `go.mod`)
- [miekg/dns](https://github.com/miekg/dns) — DNS message parsing for DoH
- [rs/zerolog](https://github.com/rs/zerolog) — structured logging
- [golang.org/x/net/proxy](https://pkg.go.dev/golang.org/x/net/proxy) — SOCKS5 dialer for upstream Tor
