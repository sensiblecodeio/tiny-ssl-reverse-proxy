package proxyprotocol

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"sync"

	"github.com/sensiblecodeio/tiny-ssl-reverse-proxy/proxyprotocol/proxyline"
)

type Accept struct {
	c   net.Conn
	err error
}

type Listener struct {
	net.Listener
	wg      sync.WaitGroup
	accepts <-chan Accept
	done    chan<- struct{}
}

func NewListener(underlying net.Listener) net.Listener {
	done := make(chan struct{})
	accepts := make(chan Accept, 10)

	l := &Listener{
		underlying,
		sync.WaitGroup{},
		accepts,
		done,
	}

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		for {
			// underlying
			c, err := underlying.Accept()
			if err != nil {
				accepts <- Accept{c, err}
				continue
			}

			// Asynchronously process a PROXY instruction before passing it off
			// to the consumer's accept loop.
			go func() {
				// Process the "PROXY" string
				buf := bufio.NewReader(c)
				p, err := proxyline.ConsumeProxyLine(buf)
				if err != nil {
					log.Printf("proxyprotocol failed to parse PROXY header: "+
						"%v", err)
					// Failed to read the proxy string, drop the connection.

					_ = c.Close() // Ignore the error.
					return
				}

				// Wrap the connection with a reader which first reads whatever
				// remains in the buffer, followed by the rest of the
				// connection.
				// Because we're using buf.Buffered(), can ignore the error.
				bufbytes, _ := buf.Peek(buf.Buffered())
				buffered := bytes.NewReader(bufbytes)
				r := io.MultiReader(buffered, c)

				wrapped := &Conn{Reader: r, Conn: c}
				if p != nil {
					ra := &net.TCPAddr{p.SrcAddr.IP, p.SrcPort, p.SrcAddr.Zone}
					la := &net.TCPAddr{p.DstAddr.IP, p.DstPort, p.DstAddr.Zone}
					wrapped.remoteAddr = ra
					wrapped.localAddr = la
				}

				accepts <- Accept{wrapped, err}
			}()

			select {
			case <-done:
				return
			default:
			}
		}
	}()

	return l
}

func (l *Listener) Accept() (net.Conn, error) {
	accept, ok := <-l.accepts
	if !ok {
		return nil, io.ErrClosedPipe
	}

	return accept.c, accept.err
}

func (l *Listener) Close() error {
	close(l.done)
	err := l.Close()
	l.wg.Wait()
	return err
}

type Conn struct {
	io.Reader
	net.Conn
	remoteAddr, localAddr net.Addr
}

func (c *Conn) Read(bs []byte) (int, error) {
	return c.Reader.Read(bs)
}

// LocalAddr returns the specified local addr, if there is one.
func (c *Conn) LocalAddr() net.Addr {
	if c.localAddr != nil {
		return c.localAddr
	}
	return c.Conn.LocalAddr()
}

// RemoteAddr returns the specified remote addr, if there is one.
func (c *Conn) RemoteAddr() net.Addr {
	if c.remoteAddr != nil {
		return c.remoteAddr
	}
	return c.Conn.RemoteAddr()
}
