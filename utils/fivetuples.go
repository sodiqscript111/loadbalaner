package utils

import (
	"net"
	"net/http"
)

type Protocol uint8

const (
	TCP Protocol = 6
	UDP Protocol = 17
)

type FiveTuple struct {
	SrcIP    net.IP
	DstIP    net.IP
	SrcPort  uint16
	DstPort  uint16
	Protocol Protocol
}

func GetFiveTuple(r *http.Request) (FiveTuple, error) {
	host, port, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return FiveTuple{}, err
	}

	srcPort, err := net.LookupPort("tcp", port)
	if err != nil {
		return FiveTuple{}, err
	}

	localAddr := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	localHost, localPort, err := net.SplitHostPort(localAddr.String())
	if err != nil {
		return FiveTuple{}, err
	}

	dstPort, err := net.LookupPort("tcp", localPort)
	if err != nil {
		return FiveTuple{}, err
	}

	return FiveTuple{
		SrcIP:    net.ParseIP(host),
		DstIP:    net.ParseIP(localHost),
		SrcPort:  uint16(srcPort),
		DstPort:  uint16(dstPort),
		Protocol: TCP,
	}, nil
}
