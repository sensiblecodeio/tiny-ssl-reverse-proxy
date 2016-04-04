package proxyprotocol

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

// Copied verbatim from net/http's server.go.
// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
}

func BehindTCPProxyListenAndServeTLS(srv *http.Server, certFile, keyFile string) error {
	// Begin copied almost verbatim from net/http
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	// Ensure we don't modify *TLSConfig, in case it is reused.
	srv.TLSConfig = cloneTLSClientConfig(srv.TLSConfig)

	err := http2.ConfigureServer(srv, nil)
	if err != nil {
		return err
	}

	foundHTTP1 := false
	for _, proto := range srv.TLSConfig.NextProtos {
		if proto == "http/1.1" {
			foundHTTP1 = true
			break
		}
	}

	if !foundHTTP1 {
		srv.TLSConfig.NextProtos = append(srv.TLSConfig.NextProtos, "http/1.1")
	}

	srv.TLSConfig.Certificates = make([]tls.Certificate, 1)
	srv.TLSConfig.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	// End copied almost verbatim from net/http

	// Wrap the listener with one understanding the PROXY protocol
	var listener net.Listener
	listener = tcpKeepAliveListener{ln.(*net.TCPListener)}
	listener = NewListener(listener)
	listener = tls.NewListener(listener, srv.TLSConfig)
	return srv.Serve(listener)
}

// ListenAndServe listens on the TCP network address srv.Addr and then
// calls Serve to handle requests on incoming connections.  If
// srv.Addr is blank, ":http" is used.
func BehindTCPProxyListenAndServe(srv *http.Server) error {
	// Begin copied verbatim from net/http
	addr := srv.Addr
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	// End copied verbatim from net/http

	// Wrap the listener with one understanding the PROXY protocol
	listener := NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)})
	return srv.Serve(listener)
}

// cloneTLSClientConfig is like cloneTLSConfig but omits
// the fields SessionTicketsDisabled and SessionTicketKey.
// This makes it safe to call cloneTLSClientConfig on a config
// in active use by a server.
// COPIED FROM net/http/transport.go
func cloneTLSClientConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{}
	}
	return &tls.Config{
		Rand:                     cfg.Rand,
		Time:                     cfg.Time,
		Certificates:             cfg.Certificates,
		NameToCertificate:        cfg.NameToCertificate,
		GetCertificate:           cfg.GetCertificate,
		RootCAs:                  cfg.RootCAs,
		NextProtos:               cfg.NextProtos,
		ServerName:               cfg.ServerName,
		ClientAuth:               cfg.ClientAuth,
		ClientCAs:                cfg.ClientCAs,
		InsecureSkipVerify:       cfg.InsecureSkipVerify,
		CipherSuites:             cfg.CipherSuites,
		PreferServerCipherSuites: cfg.PreferServerCipherSuites,
		ClientSessionCache:       cfg.ClientSessionCache,
		MinVersion:               cfg.MinVersion,
		MaxVersion:               cfg.MaxVersion,
		CurvePreferences:         cfg.CurvePreferences,
	}
}
