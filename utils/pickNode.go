package utils

import (
	"encoding/binary"
	"hash/crc32"

	"loadbalancer/backend"
)

func (t FiveTuple) Hash() uint32 {
	h := crc32.NewIEEE()

	if t.SrcIP != nil {
		h.Write(t.SrcIP)
	}
	if t.DstIP != nil {
		h.Write(t.DstIP)
	}

	var buf [5]byte
	binary.BigEndian.PutUint16(buf[0:2], t.SrcPort)
	binary.BigEndian.PutUint16(buf[2:4], t.DstPort)
	buf[4] = byte(t.Protocol)

	h.Write(buf[:])

	return h.Sum32()
}

// SelectHealthyBackend selects an active backend from the pool using 5-tuple hashing.
func SelectHealthyBackend(tuple FiveTuple, pool *backend.ServerPool) *backend.Backend {
	alive := pool.GetAliveBackends()
	if len(alive) == 0 {
		return nil
	}

	hashVal := tuple.Hash()
	selectedIdx := hashVal % uint32(len(alive))
	return alive[selectedIdx]
}

// SelectBackend selects a backend from a slice of string addresses using 5-tuple hashing.
func SelectBackend(tuple FiveTuple, backends []string) string {
	if len(backends) == 0 {
		return ""
	}

	hashVal := tuple.Hash()
	selectedIdx := hashVal % uint32(len(backends))
	return backends[selectedIdx]
}
