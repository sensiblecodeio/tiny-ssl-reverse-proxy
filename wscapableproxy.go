package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"golang.org/x/net/websocket"
)

func IsWebsocket(r *http.Request) bool {
	contains := func(headers []string, part string) bool {
		for _, value := range headers {
			for _, token := range strings.Split(value, ",") {
				if strings.EqualFold(strings.TrimSpace(token), part) {
					return true
				}
			}
		}
		return false
	}

	if !contains(r.Header["Connection"], "upgrade") {
		return false
	}

	if !contains(r.Header["Upgrade"], "websocket") {
		return false
	}

	return true
}

type WebsocketCapableReverseProxy struct {
	*httputil.ReverseProxy
}

func NewWebsocketCapableReverseProxy(url *url.URL) *WebsocketCapableReverseProxy {
	return &WebsocketCapableReverseProxy{
		httputil.NewSingleHostReverseProxy(url),
	}
}

func (p *WebsocketCapableReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if IsWebsocket(r) {
		websocket.Handler(p.ServeWebsocket).ServeHTTP(w, r)
	} else {
		p.ReverseProxy.ServeHTTP(w, r)
	}
}

func (p *WebsocketCapableReverseProxy) ServeWebsocket(conn *websocket.Conn) {

	transport := p.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	outreq := new(http.Request)
	r := conn.Request()
	*outreq = *r // includes shallow copies of maps, but okay

	p.Director(outreq)

	switch outreq.URL.Scheme {
	case "http", "":
		outreq.URL.Scheme = "ws"
	case "https":
		outreq.URL.Scheme = "wss"
	}

	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		if prior, ok := outreq.Header["X-Forwarded-For"]; ok {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		outreq.Header.Set("X-Forwarded-For", clientIP)
	}

	originConfig := &websocket.Config{Version: websocket.ProtocolVersionHybi13}
	origin, _ := websocket.Origin(originConfig, r)

	config := &websocket.Config{
		Location: outreq.URL,
		Origin:   origin,
		Header:   outreq.Header,
		Version:  websocket.ProtocolVersionHybi13,
	}

	srv, err := websocket.DialConfig(config)
	if err != nil {
		log.Printf("Bad gateway: %v", err)
		return
	}

	cp := func(dst io.WriteCloser, src io.Reader) {
		// Ignore copy errors, likely to be a disconnect.
		defer dst.Close()
		_, _ = io.Copy(dst, src)
	}

	finish := make(chan struct{})
	defer func() { <-finish }()

	go func() {
		defer close(finish)
		cp(srv, conn)
	}()

	cp(conn, srv)
}
