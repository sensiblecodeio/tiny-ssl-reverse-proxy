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

func NewWebsocketCapableReverseProxy(
	proxy *httputil.ReverseProxy, url *url.URL,
) *WebsocketCapableReverseProxy {
	return &WebsocketCapableReverseProxy{proxy, url}
}

func (p *WebsocketCapableReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if IsWebsocket(r) {
		p.ServeWebsocket(w, r)
	} else {
		p.ReverseProxy.ServeHTTP(w, r)
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",

	// Headers used in Websocket (by inspection)
	"Sec-Websocket-Key",
	"Sec-Websocket-Version",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (p *WebsocketCapableReverseProxy) ServeWebsocket(w http.ResponseWriter, r *http.Request) {

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

	// Avoid duplicating the hop-by-hop headers.
	// Copied from:
	// https://github.com/golang/go/blob/b83b01110090c41fc24750ecabf0b87c5fbff233/src/net/http/httputil/reverseproxy.go#L164-L179
	copiedHeaders := false
	for _, h := range hopHeaders {
		if outreq.Header.Get(h) != "" {
			if !copiedHeaders {
				outreq.Header = make(http.Header)
				copyHeader(outreq.Header, r.Header)
				copiedHeaders = true
			}
			outreq.Header.Del(h)
		}
	}

	log.Printf("Establishing outbound websocket to %v", outreq.URL.String())

	dial := websocket.DefaultDialer.Dial
	outConn, resp, err := dial(outreq.URL.String(), outreq.Header)
	if err != nil {
		if resp != nil {
			log.Printf("outbound websocket dial error, status: %v, err: %v",
				resp.StatusCode, err)
			w.WriteHeader(resp.StatusCode)
			_, err := io.Copy(w, resp.Body)
			if err != nil {
				log.Printf("error copying outbound body to response. err: %v", err)
			}
		} else {
			log.Printf("outbound websocket dial error, err: %v", err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		}
		return
	}
	defer outConn.Close()

	inConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade: %v", err)
		// Don't send any response here, Upgrade already does that on error.
		return
	}
	defer inConn.Close()

	rawIn := inConn.UnderlyingConn()
	rawOut := outConn.UnderlyingConn()

	go func() {
		_, _ = io.Copy(rawOut, rawIn)
	}()

	_, _ = io.Copy(rawIn, rawOut)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}
