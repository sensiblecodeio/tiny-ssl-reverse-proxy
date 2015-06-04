package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	auth "github.com/abbot/go-http-auth"

	"github.com/scraperwiki/tiny-ssl-reverse-proxy/proxyprotocol"
)

var message = `<!DOCTYPE html><html>
<style>
body {
	font-family: fantasy;
	text-align: center;
	padding-top: 20%;
	background-color: #f1f6f8;
}
</style>
<body>
<h1>503 Backend Unavailable</h1>
<p>Sorry, we&lsquo;re having a brief problem. You can retry.</p>
<p>If the problem persists, please get in touch.</p>
</body>
</html>`

type ConnectionErrorHandler struct{ http.RoundTripper }

func (c *ConnectionErrorHandler) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.RoundTripper.RoundTrip(req)
	if _, ok := err.(*net.OpError); ok {
		r := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       ioutil.NopCloser(bytes.NewBufferString(message)),
		}
		return r, nil
	}
	return resp, err
}

type SubnetRoute struct {
	inside, outside http.Handler
	*net.IPNet
}

func (s *SubnetRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	ip := net.ParseIP(host)
	if ip == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if s.Contains(ip) {
		s.inside.ServeHTTP(w, r)
	} else {
		s.outside.ServeHTTP(w, r)
	}
}

func main() {
	var (
		listen, cert, key, htpasswdFile, where, subnet string
		tlsFlag, behindTCPProxy                        bool
	)
	flag.StringVar(&listen, "listen", ":443", "Bind address to listen on")
	flag.StringVar(&key, "key", "/etc/ssl/private/key.pem", "Path to PEM key")
	flag.StringVar(&cert, "cert", "/etc/ssl/private/cert.pem", "Path to PEM certificate")
	flag.StringVar(&htpasswdFile, "htpasswdFile", "", "File to use for htpasswd protected access")
	flag.StringVar(&where, "where", "http://localhost:80", "Place to forward connections to")
	flag.StringVar(&subnet, "subnet", "", "If specified, subnet which can circumvent htpasswd authorization")
	flag.BoolVar(&tlsFlag, "tls", true, "accept HTTPS connections")
	flag.BoolVar(&behindTCPProxy, "behind-tcp-proxy", false, "running behind TCP proxy (such as ELB or HAProxy)")
	flag.Parse()

	url, err := url.Parse(where)
	if err != nil {
		log.Fatalln("Fatal parsing -where:", err)
	}

	var ipNet *net.IPNet
	if subnet != "" {
		_, ipNet, err = net.ParseCIDR(subnet)
		if err != nil {
			log.Fatalln("Error parsing -subnet:", err)
		}
	}

	httpProxy := httputil.NewSingleHostReverseProxy(url)
	httpProxy.Transport = &ConnectionErrorHandler{http.DefaultTransport}

	proxy := NewWebsocketCapableReverseProxy(httpProxy, url)

	var handler, authenticated http.Handler

	handler = proxy

	originalHandler := handler
	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Add("X-Forwarded-Proto", "https")
		originalHandler.ServeHTTP(w, r)
	})

	// Only really authenticated if htpasswdFile is specified
	authenticated = handler

	if htpasswdFile != "" {
		// First check that the htpasswdFile exists and is readable
		fd, err := os.Open(htpasswdFile)
		if err != nil {
			log.Fatalln("Error opening htpasswdFile:", err)
		}
		fd.Close()

		secrets := auth.HtpasswdFileProvider(htpasswdFile)
		authenticator := auth.NewBasicAuthenticator(where, secrets)
		authenticated = auth.JustCheck(authenticator, handler.ServeHTTP)
	}

	if subnet != "" {
		handler = &SubnetRoute{handler, authenticated, ipNet}
	} else {
		handler = authenticated
	}

	config := &tls.Config{
		MinVersion: tls.VersionTLS10,
		CipherSuites: []uint16{
			// Note: RC4 is removed.
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		},
	}

	handler = &LoggingMiddleware{handler}

	server := &http.Server{Addr: listen, Handler: handler, TLSConfig: config}

	switch {
	case tlsFlag && behindTCPProxy:
		err = proxyprotocol.BehindTCPProxyListenAndServeTLS(server, cert, key)
	case behindTCPProxy:
		err = proxyprotocol.BehindTCPProxyListenAndServe(server)
	case tlsFlag:
		err = server.ListenAndServeTLS(cert, key)
	default:
		err = server.ListenAndServe()
	}

	log.Fatalln(err)
}
