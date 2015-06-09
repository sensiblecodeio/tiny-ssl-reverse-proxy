package proxyline

import (
	"bufio"
	"net"
	"strings"
	"testing"
)

var (
	fixtureTCP4 = "PROXY TCP4 127.0.0.1 127.0.0.1 65533 65533\r\n"
	fixtureTCP6 = "PROXY TCP6 2001:4801:7817:72:d4d9:211d:ff10:1631 2001:4801:7817:72:d4d9:211d:ff10:1631 65533 65533\r\n"

	v4addr, _ = net.ResolveIPAddr("ip", "127.0.0.1")
	v6addr, _ = net.ResolveIPAddr("ip", "2001:4801:7817:72:d4d9:211d:ff10:1631")
	pTCP4     = &ProxyLine{Protocol: TCP4, SrcAddr: v4addr, DstAddr: v4addr, SrcPort: 65533, DstPort: 65533}
	pTCP6     = &ProxyLine{Protocol: TCP6, SrcAddr: v6addr, DstAddr: v6addr, SrcPort: 65533, DstPort: 65533}

	invalidProxyLines = []string{
		"PROXY TCP4 127.0.0.1 127.0.0.1 65533 65533", // no CRLF
		"PROXY \r\n",                                 // not enough fields
		"PROXY TCP6 127.0.0.1 127.0.0.1 65533 65533\r\n,",                                                        // unmatched protocol addr
		"PROXY TCP4 2001:4801:7817:72:d4d9:211d:ff10:1631 2001:4801:7817:72:d4d9:211d:ff10:1631 65533 65533\r\n", // unmatched protocol addr
	}
	noneProxyLine = "There is no spoon."
)

func TestParseTCP4(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(fixtureTCP4))
	p, err := ConsumeProxyLine(reader)
	if err != nil {
		t.Fatalf("Parsing TCP4 failed: %v\n", err)
	}
	if !p.EqualTo(pTCP4) {
		t.Fatalf("Expected ProxyLine %v, got %v\n", pTCP4, p)
	}
}

func TestParseTCP6(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(fixtureTCP6))
	p, err := ConsumeProxyLine(reader)
	if err != nil {
		t.Fatalf("Parsing TCP6 failed: %v\n", err)
	}
	if !p.EqualTo(pTCP6) {
		t.Fatalf("Expected ProxyLine %v, got %v\n", pTCP6, p)
	}
}

func TestParseNonProxyLine(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(noneProxyLine))
	p, err := ConsumeProxyLine(reader)
	if err != nil || p != nil {
		t.Fatalf("Parsing none PROXY line failed. Expected nil, nil; got %q, %q\n", p, err)
	}
}

func TestInvalidProxyLines(t *testing.T) {
	for _, str := range invalidProxyLines {
		reader := bufio.NewReader(strings.NewReader(str))
		_, err := ConsumeProxyLine(reader)
		if err == nil {
			t.Fatalf("Parsing an invalid PROXY line %q fails to fail\n", str)
		}
	}
}

func (p *ProxyLine) EqualTo(q *ProxyLine) bool {
	return p.Protocol == q.Protocol &&
		p.SrcAddr.String() == q.SrcAddr.String() &&
		p.DstAddr.String() == q.DstAddr.String() &&
		p.SrcPort == q.SrcPort &&
		p.DstPort == q.DstPort
}
