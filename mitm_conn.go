package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
)

type peekConn struct {
	net.Conn
	r io.Reader
}

func (c *peekConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}

type connResponseWriter struct {
	request     *http.Request
	conn        net.Conn
	header      http.Header
	wroteHeader bool
	buf         *bufio.ReadWriter
}

func newConnResponseWriter(conn net.Conn, req *http.Request) *connResponseWriter {
	return &connResponseWriter{
		conn:    conn,
		header:  http.Header{},
		request: req,
		buf:     bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)),
	}
}

func (w *connResponseWriter) Header() http.Header {
	return w.header
}

func (w *connResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	fmt.Fprintf(w.conn, "HTTP/1.1 %d %s\r\n", code, http.StatusText(code))
	w.header.Write(w.conn)
	fmt.Fprint(w.conn, "\r\n")
}

func (w *connResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.conn.Write(b)
}

func (w *connResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.buf, nil
}
