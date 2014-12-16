package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	auth "github.com/abbot/go-http-auth"
)

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
	)
	flag.StringVar(&listen, "listen", ":443", "Bind address to listen on")
	flag.StringVar(&key, "key", "/etc/ssl/private/key.pem", "Path to PEM key")
	flag.StringVar(&cert, "cert", "/etc/ssl/private/cert.pem", "Path to PEM certificate")
	flag.StringVar(&htpasswdFile, "htpasswdFile", "", "File to use for htpasswd protected access")
	flag.StringVar(&where, "where", "http://localhost:80", "Place to forward connections to")
	flag.StringVar(&subnet, "subnet", "", "If specified, subnet which can circumvent htpasswd authorization")
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

	var handler, authenticated http.Handler
	handler = httputil.NewSingleHostReverseProxy(url)

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
	}

	err = http.ListenAndServeTLS(listen, cert, key, handler)
	if err != nil {
		log.Fatalln("http.ListenAndServeTLS:", err)
	}
}
