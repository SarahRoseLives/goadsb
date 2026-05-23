package main

import (
	"math"
	"sync"
)

const (
	MagBufSize    = 16 * 16384
	MagBufOverlap = 2048
)

var magLUT [256][256]uint16

func init() {
	for i := 0; i < 256; i++ {
		for q := 0; q < 256; q++ {
			si := float64(int(i) - 127)
			sq := float64(int(q) - 127)
			mag := math.Sqrt(si*si + sq*sq) * 360.0
			if mag > 65535 {
				mag = 65535
			}
			magLUT[i][q] = uint16(mag)
		}
	}
}

type MagRing struct {
	mu     sync.Mutex
	buf    []uint16
	write  int
	length int
	total  uint64
}

func NewMagRing() *MagRing {
	return &MagRing{
		buf: make([]uint16, MagBufSize),
	}
}

func (r *MagRing) Write(iqSamples []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := 0; i < len(iqSamples); i += 2 {
		ival := iqSamples[i]
		qval := iqSamples[i+1]
		mag := magLUT[ival][qval]
		r.buf[r.write] = mag
		r.write = (r.write + 1) % MagBufSize
		r.length++
		if r.length > MagBufSize {
			r.length = MagBufSize
		}
		r.total++
	}
}

func (r *MagRing) CopySafe() ([]uint16, int, uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	overlap := MagBufOverlap
	if r.length < overlap*2 {
		return nil, 0, r.total
	}

	readable := r.length - overlap
	if readable <= 0 {
		return nil, 0, r.total
	}

	start := (r.write - r.length + MagBufSize) % MagBufSize
	out := make([]uint16, r.length)

	if start+r.length <= MagBufSize {
		copy(out, r.buf[start:start+r.length])
	} else {
		n1 := MagBufSize - start
		n2 := r.length - n1
		copy(out, r.buf[start:])
		copy(out[n1:], r.buf[:n2])
	}

	r.length = overlap

	return out, readable, r.total
}
