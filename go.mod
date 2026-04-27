module socks5-server

go 1.26.1

require (
	github.com/coder/websocket v1.8.14
	github.com/miekg/dns v1.1.72
	github.com/refraction-networking/utls v1.8.2
	github.com/rs/zerolog v1.35.0
	github.com/things-go/go-socks5 v0.0.0-20260412185445-80f855dd35d3
	golang.org/x/net v0.52.0
)

require (
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/crypto v0.49.0 // indirect
	golang.org/x/mod v0.33.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	golang.org/x/tools v0.42.0 // indirect
)

replace github.com/things-go/go-socks5 => github.com/imkk000/go-socks5 v0.0.0-20260412185445-80f855dd35d3
