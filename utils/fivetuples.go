package utils

import (
	"net"
	"strconv"
	"strings"
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

func GetFiveTuple(conn net.Conn) (FiveTuple, error) {
	srcHost, srcPortStr, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		return FiveTuple{}, err
	}

	srcPort, err := strconv.ParseUint(srcPortStr, 10, 16)
	if err != nil {
		return FiveTuple{}, err
	}

	dstHost, dstPortStr, err := net.SplitHostPort(conn.LocalAddr().String())
	if err != nil {
		return FiveTuple{}, err
	}

	dstPort, err := strconv.ParseUint(dstPortStr, 10, 16)
	if err != nil {
		return FiveTuple{}, err
	}

	protocol := TCP
	if strings.HasPrefix(conn.RemoteAddr().Network(), "udp") {
		protocol = UDP
	}

	return FiveTuple{
		SrcIP:    net.ParseIP(srcHost),
		DstIP:    net.ParseIP(dstHost),
		SrcPort:  uint16(srcPort),
		DstPort:  uint16(dstPort),
		Protocol: protocol,
	}, nil
}
