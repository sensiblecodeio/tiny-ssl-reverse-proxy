package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	auth "github.com/abbot/go-http-auth"
)

func main() {
	var (
		listen, cert, key, htpasswdFile, where string
	)
	flag.StringVar(&listen, "listen", ":443", "Bind address to listen on")
	flag.StringVar(&key, "key", "/etc/ssl/private/key.pem", "Path to PEM key")
	flag.StringVar(&cert, "cert", "/etc/ssl/private/cert.pem", "Path to PEM certificate")
	flag.StringVar(&htpasswdFile, "htpasswdFile", "", "File to use for htpasswd protected access")
	flag.StringVar(&where, "where", "http://localhost:80", "Place to forward connections to")

	flag.Parse()
	url, err := url.Parse(where)
	if err != nil {
		log.Fatal("Fatal parsing -where:", err)
	}

	var handler http.Handler = httputil.NewSingleHostReverseProxy(url)

	if htpasswdFile != "" {
		fd, err := os.Open(htpasswdFile)
		if err != nil {
			log.Fatal("Error opening htpasswdFile:", err)
		}
		fd.Close()
		secrets := auth.HtpasswdFileProvider(htpasswdFile)
		authenticator := auth.NewBasicAuthenticator(where, secrets)
		handler = auth.JustCheck(authenticator, handler.ServeHTTP)
	}

	err = http.ListenAndServeTLS(listen, cert, key, handler)
	if err != nil {
		log.Fatal("http.ListenAndServeTLS:", err)
	}
}
