# Go Socks5 Server

## Why?

Migrate from `dns over https` query into full socks5 proxy with block some website and using proxy to pass through censorship. Block websites use `tor proxy` to navigate, but normal websites will use default proxy (no proxy) to navigate. I also use `mkcert` to be man-in-the-middle (MITM) between server and client. I just want allow CORS from any website.

## Design

From this design I can block some request and decrypt request between victim and request server to inject something then response to victim.
I will generate certificate with local root CA automatically when cert is not exists into cache.

### Steps

1. client (installed trusted CA)
2. connect through socks5
3. resolved dns inside (tordns)
4. forward dial into fake https server
5. generate new cert
6. add generated cert into cache
7. forward request into real website as reverse proxy
