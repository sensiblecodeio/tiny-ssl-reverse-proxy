package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
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

	target *url.URL
}

func NewWebsocketCapableReverseProxy(url *url.URL) *WebsocketCapableReverseProxy {
	return &WebsocketCapableReverseProxy{
		httputil.NewSingleHostReverseProxy(url),
		url,
	}
}

func (p *WebsocketCapableReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if IsWebsocket(r) {
		WebsocketHandlerFunc(p.ServeWebsocket).ServeHTTP(w, r)
	} else {
		p.ReverseProxy.ServeHTTP(w, r)
	}
}

func (p *WebsocketCapableReverseProxy) ServeWebsocket(inConn *websocket.Conn, r *http.Request) {

	defer inConn.Close()

	transport := p.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	outreq := new(http.Request)
	*outreq = *r // includes shallow copies of maps, but okay

	p.Director(outreq)

	// Note: Director rewrites outreq.URL.Host, but we need it to be the
	// internal host for the websocket dial. The Host: header gets set to the
	// inbound http request's `Host` header.
	outreq.URL.Host = p.target.Host

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

	outreq.Header.Set("Host", r.Host)

	log.Printf("Establishing outbound websocket to %v", outreq.URL.String())

	dial := websocket.DefaultDialer.Dial
	outConn, resp, err := dial(outreq.URL.String(), outreq.Header)
	if err != nil {
		if resp != nil {
			log.Printf("outbound websocket dial error, status: %v, err: %v",
				resp.StatusCode, err)
		} else {
			log.Printf("outbound websocket dial error, err: %v", err)
		}
		return
	}
	defer outConn.Close()

	finish := make(chan struct{})
	defer func() { <-finish }()

	rawIn := inConn.UnderlyingConn()
	rawOut := outConn.UnderlyingConn()

	go func() {
		defer close(finish)
		_, _ = io.Copy(rawOut, rawIn)
	}()

	_, _ = io.Copy(rawIn, rawOut)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WebsocketHandlerFunc func(*websocket.Conn, *http.Request)

func (wrapped WebsocketHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade: %v", err)
		// Don't send any response here, Upgrade already does that on error.
		return
	}

	wrapped(conn, r)
}
