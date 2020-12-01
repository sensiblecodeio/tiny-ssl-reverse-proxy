package proxyprotocol

import (
	"crypto/tls"
	"net"
	"net/http"
)

func BehindTCPProxyListenAndServeTLS(srv *http.Server, certFile, keyFile string) error {
	// Begin copied almost verbatim from net/http
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	// Ensure we don't modify *TLSConfig, in case it is reused.
	srv.TLSConfig = cloneTLSClientConfig(srv.TLSConfig)

	srv.TLSConfig.NextProtos = append(srv.TLSConfig.NextProtos, "h2")

	var err error
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
	listener = ln.(*net.TCPListener)
	listener = NewListener(listener)
	listener = tls.NewListener(listener, srv.TLSConfig)
	return srv.Serve(listener)
}

// BehindTCPProxyListenAndServe listens on the TCP network address srv.Addr and then
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
	listener := NewListener(ln.(*net.TCPListener))
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
