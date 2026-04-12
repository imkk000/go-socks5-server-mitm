module socks5-server

go 1.26.1

require (
	github.com/miekg/dns v1.1.72
	github.com/rs/zerolog v1.35.0
	github.com/things-go/go-socks5 v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.52.0
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
)

replace github.com/things-go/go-socks5 => github.com/imkk000/go-socks5 v0.0.0-20260412153100-996a7fd4e66a
