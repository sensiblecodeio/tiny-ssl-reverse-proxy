package proxyprotocol

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
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
	// Begin copied verbatim from net/http
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}
	config := &tls.Config{}
	if srv.TLSConfig != nil {
		*config = *srv.TLSConfig
	}
	if config.NextProtos == nil {
		config.NextProtos = []string{"http/1.1"}
	}

	var err error
	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], err = tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	// End copied verbatim from net/http

	// Wrap the listener with one understanding the PROXY protocol
	var listener net.Listener
	listener = tcpKeepAliveListener{ln.(*net.TCPListener)}
	listener = NewListener(listener)
	listener = tls.NewListener(listener, config)
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
