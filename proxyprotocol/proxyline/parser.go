/**
 *  Copyright 2013 Rackspace
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

// Packet proxyProtocol implements Proxy Protocol parser and writer.
package proxyline

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
)

// INET protocol and family
const (
	TCP4    = "TCP4"    // TCP over IPv4
	TCP6    = "TCP6"    // TCP over IPv6
	UNKNOWN = "UNKNOWN" // Unsupported or unknown protocols
)

var (
	InvalidProxyLine   = errors.New("Invalid proxy line")
	UnmatchedIPAddress = errors.New("IP address(es) unmatched with protocol")
	InvalidPortNum     = errors.New(fmt.Sprintf("Invalid port number parsed. (expected [%d..%d])", _port_lower, _port_upper))
)

var (
	_proxy      = []byte{'P', 'R', 'O', 'X', 'Y'}
	_CRLF       = "\r\n"
	_sep        = " "
	_port_lower = 0
	_port_upper = 65535
)

type ProxyLine struct {
	Protocol string
	SrcAddr  *net.IPAddr
	DstAddr  *net.IPAddr
	SrcPort  int
	DstPort  int
}

// ConsumeProxyLine looks for PROXY line in the reader and try to parse it if found.
//
// If first 5 bytes in reader is "PROXY", the function reads one line (until first '\n') from reader and try to parse it as ProxyLine. A newly allocated ProxyLine is returned if parsing secceeds. If parsing fails, a nil and an error is returned;
//
// If first 5 bytes in reader is not "PROXY", the function simply returns (nil, nil), leaving reader intact (nothing from reader is consumed).
//
// If the being parsed PROXY line is using an unknown protocol, ConsumeProxyLine parses remaining fields as same syntax as a supported protocol assuming IP is used in layer 3, and reports error if failed.
func ConsumeProxyLine(reader *bufio.Reader) (*ProxyLine, error) {
	word, err := reader.Peek(5)
	if !bytes.Equal(word, _proxy) {
		return nil, nil
	}
	line, err := reader.ReadString('\n')
	if !strings.HasSuffix(line, _CRLF) {
		return nil, InvalidProxyLine
	}
	tokens := strings.Split(line[:len(line)-2], _sep)
	ret := new(ProxyLine)
	if len(tokens) < 6 {
		return nil, InvalidProxyLine
	}
	switch tokens[1] {
	case TCP4:
		ret.Protocol = TCP4
	case TCP6:
		ret.Protocol = TCP6
	default:
		ret.Protocol = UNKNOWN
	}
	ret.SrcAddr, err = parseIPAddr(ret.Protocol, tokens[2])
	if err != nil {
		return nil, err
	}
	ret.DstAddr, err = parseIPAddr(ret.Protocol, tokens[3])
	if err != nil {
		return nil, err
	}
	ret.SrcPort, err = parsePortNumber(tokens[4])
	if err != nil {
		return nil, err
	}
	ret.DstPort, err = parsePortNumber(tokens[5])
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// WriteProxyLine formats p as valid PROXY line into w
func (p *ProxyLine) WriteProxyLine(w io.Writer) (err error) {
	_, err = fmt.Fprintf(w, "PROXY %s %s %s %d %d\r\n", p.Protocol, p.SrcAddr.String(), p.DstAddr.String(), p.SrcPort, p.DstPort)
	return
}

func parsePortNumber(portStr string) (port int, err error) {
	port, err = strconv.Atoi(portStr)
	if err == nil {
		if port < _port_lower || port > _port_upper {
			err = InvalidPortNum
		}
	}
	return
}

func parseIPAddr(protocol string, addrStr string) (addr *net.IPAddr, err error) {
	proto := "ip"
	if protocol == TCP4 {
		proto = "ip4"
	} else if protocol == TCP6 {
		proto = "ip6"
	}
	addr, err = net.ResolveIPAddr(proto, addrStr)
	if err == nil {
		tryV4 := addr.IP.To4()
		if (protocol == TCP4 && tryV4 == nil) || (protocol == TCP6 && tryV4 != nil) {
			err = UnmatchedIPAddress
		}
	}
	return
}
