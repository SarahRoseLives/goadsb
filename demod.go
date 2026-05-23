package main

const (
	MODES_LONG_MSG_BITS  = 112
	MODES_LONG_MSG_BYTES = 14
	MODES_SHORT_MSG_BITS = 56
	MODES_SHORT_MSG_BYTES = 7
	MODES_PREAMBLE_US    = 8
)

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func slicePhase0(m []uint16) int {
	return 5*int(m[0]) - 3*int(m[1]) - 2*int(m[2])
}

func slicePhase1(m []uint16) int {
	return 4*int(m[0]) - int(m[1]) - 3*int(m[2])
}

func slicePhase2(m []uint16) int {
	return 3*int(m[0]) + int(m[1]) - 4*int(m[2])
}

func slicePhase3(m []uint16) int {
	return 2*int(m[0]) + 3*int(m[1]) - 5*int(m[2])
}

func slicePhase4(m []uint16) int {
	return int(m[0]) + 5*int(m[1]) - 5*int(m[2]) - int(m[3])
}

func correlatePhase0(m []uint16) int { return slicePhase0(m) * 26 }
func correlatePhase1(m []uint16) int { return slicePhase1(m) * 38 }
func correlatePhase2(m []uint16) int { return slicePhase2(m) * 38 }
func correlatePhase3(m []uint16) int { return slicePhase3(m) * 26 }
func correlatePhase4(m []uint16) int { return slicePhase4(m) * 19 }

func correlateCheck0(m []uint16) int {
	return absInt(correlatePhase0(m[0:])) +
		absInt(correlatePhase2(m[2:])) +
		absInt(correlatePhase4(m[4:])) +
		absInt(correlatePhase1(m[7:])) +
		absInt(correlatePhase3(m[9:]))
}

func correlateCheck1(m []uint16) int {
	return absInt(correlatePhase1(m[0:])) +
		absInt(correlatePhase3(m[2:])) +
		absInt(correlatePhase0(m[5:])) +
		absInt(correlatePhase2(m[7:])) +
		absInt(correlatePhase4(m[9:]))
}

func correlateCheck2(m []uint16) int {
	return absInt(correlatePhase2(m[0:])) +
		absInt(correlatePhase4(m[2:])) +
		absInt(correlatePhase1(m[5:])) +
		absInt(correlatePhase3(m[7:])) +
		absInt(correlatePhase0(m[10:]))
}

func correlateCheck3(m []uint16) int {
	return absInt(correlatePhase3(m[0:])) +
		absInt(correlatePhase0(m[3:])) +
		absInt(correlatePhase2(m[5:])) +
		absInt(correlatePhase4(m[7:])) +
		absInt(correlatePhase1(m[10:]))
}

func correlateCheck4(m []uint16) int {
	return absInt(correlatePhase4(m[0:])) +
		absInt(correlatePhase1(m[3:])) +
		absInt(correlatePhase3(m[5:])) +
		absInt(correlatePhase0(m[8:])) +
		absInt(correlatePhase2(m[10:]))
}

func bestPhase(m []uint16) int {
	best := -1
	bestval := int(m[0]) + int(m[1]) + int(m[2]) + int(m[3]) + int(m[4]) + int(m[5])

	if test := correlateCheck4(m[0:]); test > bestval {
		bestval = test
		best = 4
	}
	if test := correlateCheck0(m[1:]); test > bestval {
		bestval = test
		best = 5
	}
	if test := correlateCheck1(m[1:]); test > bestval {
		bestval = test
		best = 6
	}
	if test := correlateCheck2(m[1:]); test > bestval {
		bestval = test
		best = 7
	}
	if test := correlateCheck3(m[1:]); test > bestval {
		bestval = test
		best = 8
	}
	return best
}

type RawMessage struct {
	Data     [MODES_LONG_MSG_BYTES]byte
	Bits     int
	Score    int
	SigLevel float64
}

func demodulate2400(mag []uint16, mlen int, phaseEnhance bool) []RawMessage {
	var results []RawMessage
	msg1 := make([]byte, MODES_LONG_MSG_BYTES)
	msg2 := make([]byte, MODES_LONG_MSG_BYTES)

	for j := 0; j+20 <= mlen; j++ {
		preamble := mag[j:]

		if !(preamble[0] < preamble[1] && preamble[12] > preamble[13]) {
			continue
		}

		var high int
		var baseSignal, baseNoise uint32

		switch {
		case preamble[1] > preamble[2] && preamble[2] < preamble[3] && preamble[3] > preamble[4] &&
			preamble[8] < preamble[9] && preamble[9] > preamble[10] && preamble[10] < preamble[11]:
			high = (int(preamble[1]) + int(preamble[3]) + int(preamble[9]) + int(preamble[11]) + int(preamble[12])) / 4
			baseSignal = uint32(preamble[1] + preamble[3] + preamble[9])
			baseNoise = uint32(preamble[5] + preamble[6] + preamble[7])

		case preamble[1] > preamble[2] && preamble[2] < preamble[3] && preamble[3] > preamble[4] &&
			preamble[8] < preamble[9] && preamble[9] > preamble[10] && preamble[11] < preamble[12]:
			high = (int(preamble[1]) + int(preamble[3]) + int(preamble[9]) + int(preamble[12])) / 4
			baseSignal = uint32(preamble[1] + preamble[3] + preamble[9] + preamble[12])
			baseNoise = uint32(preamble[5] + preamble[6] + preamble[7] + preamble[8])

		case preamble[1] > preamble[2] && preamble[2] < preamble[3] && preamble[4] > preamble[5] &&
			preamble[8] < preamble[9] && preamble[10] > preamble[11] && preamble[11] < preamble[12]:
			high = (int(preamble[1]) + int(preamble[3]) + int(preamble[4]) + int(preamble[9]) + int(preamble[10]) + int(preamble[12])) / 4
			baseSignal = uint32(preamble[1] + preamble[12])
			baseNoise = uint32(preamble[6] + preamble[7])

		case preamble[1] > preamble[2] && preamble[3] < preamble[4] && preamble[4] > preamble[5] &&
			preamble[9] < preamble[10] && preamble[10] > preamble[11] && preamble[11] < preamble[12]:
			high = (int(preamble[1]) + int(preamble[4]) + int(preamble[10]) + int(preamble[12])) / 4
			baseSignal = uint32(preamble[1] + preamble[4] + preamble[10] + preamble[12])
			baseNoise = uint32(preamble[5] + preamble[6] + preamble[7] + preamble[8])

		case preamble[2] > preamble[3] && preamble[3] < preamble[4] && preamble[4] > preamble[5] &&
			preamble[9] < preamble[10] && preamble[10] > preamble[11] && preamble[11] < preamble[12]:
			high = (int(preamble[1]) + int(preamble[2]) + int(preamble[4]) + int(preamble[10]) + int(preamble[12])) / 4
			baseSignal = uint32(preamble[4] + preamble[10] + preamble[12])
			baseNoise = uint32(preamble[6] + preamble[7] + preamble[8])

		default:
			continue
		}

		if baseSignal*2 < 3*baseNoise {
			continue
		}

		if preamble[5] >= uint16(high) || preamble[6] >= uint16(high) || preamble[7] >= uint16(high) ||
			preamble[8] >= uint16(high) || preamble[14] >= uint16(high) || preamble[15] >= uint16(high) ||
			preamble[16] >= uint16(high) || preamble[17] >= uint16(high) || preamble[18] >= uint16(high) {
			continue
		}

		firstPhase, lastPhase := 0, 0
		if phaseEnhance {
			firstPhase, lastPhase = 4, 8
		} else {
			ip := bestPhase(preamble[19:])
			if ip < 0 {
				continue
			}
			firstPhase, lastPhase = ip, ip
		}

		bestMsg := msg1
		bestScore := -1
		useMsg1 := true

		for tryPhase := firstPhase; tryPhase <= lastPhase; tryPhase++ {
			var curMsg []byte
			if useMsg1 {
				curMsg = msg1
			} else {
				curMsg = msg2
			}

			phasePtr := mag[j+19+tryPhase/5:]
			phase := tryPhase % 5
			bytelen := MODES_LONG_MSG_BYTES

			for i := 0; i < bytelen; i++ {
				var b byte
				switch phase {
				case 0:
					b = bit(slicePhase0(phasePtr), 7) | bit(slicePhase2(phasePtr[2:]), 6) |
						bit(slicePhase4(phasePtr[4:]), 5) | bit(slicePhase1(phasePtr[7:]), 4) |
						bit(slicePhase3(phasePtr[9:]), 3) | bit(slicePhase0(phasePtr[12:]), 2) |
						bit(slicePhase2(phasePtr[14:]), 1) | bit(slicePhase4(phasePtr[16:]), 0)
					phase = 1
					phasePtr = phasePtr[19:]

				case 1:
					b = bit(slicePhase1(phasePtr), 7) | bit(slicePhase3(phasePtr[2:]), 6) |
						bit(slicePhase0(phasePtr[5:]), 5) | bit(slicePhase2(phasePtr[7:]), 4) |
						bit(slicePhase4(phasePtr[9:]), 3) | bit(slicePhase1(phasePtr[12:]), 2) |
						bit(slicePhase3(phasePtr[14:]), 1) | bit(slicePhase0(phasePtr[17:]), 0)
					phase = 2
					phasePtr = phasePtr[19:]

				case 2:
					b = bit(slicePhase2(phasePtr), 7) | bit(slicePhase4(phasePtr[2:]), 6) |
						bit(slicePhase1(phasePtr[5:]), 5) | bit(slicePhase3(phasePtr[7:]), 4) |
						bit(slicePhase0(phasePtr[10:]), 3) | bit(slicePhase2(phasePtr[12:]), 2) |
						bit(slicePhase4(phasePtr[14:]), 1) | bit(slicePhase1(phasePtr[17:]), 0)
					phase = 3
					phasePtr = phasePtr[19:]

				case 3:
					b = bit(slicePhase3(phasePtr), 7) | bit(slicePhase0(phasePtr[3:]), 6) |
						bit(slicePhase2(phasePtr[5:]), 5) | bit(slicePhase4(phasePtr[7:]), 4) |
						bit(slicePhase1(phasePtr[10:]), 3) | bit(slicePhase3(phasePtr[12:]), 2) |
						bit(slicePhase0(phasePtr[15:]), 1) | bit(slicePhase2(phasePtr[17:]), 0)
					phase = 4
					phasePtr = phasePtr[19:]

				case 4:
					b = bit(slicePhase4(phasePtr), 7) | bit(slicePhase1(phasePtr[3:]), 6) |
						bit(slicePhase3(phasePtr[5:]), 5) | bit(slicePhase0(phasePtr[8:]), 4) |
						bit(slicePhase2(phasePtr[10:]), 3) | bit(slicePhase4(phasePtr[12:]), 2) |
						bit(slicePhase1(phasePtr[15:]), 1) | bit(slicePhase3(phasePtr[17:]), 0)
					phase = 0
					phasePtr = phasePtr[20:]
				}

				curMsg[i] = b

				if i == 0 {
					df := b >> 3
					switch df {
					case 0, 4, 5, 11:
						bytelen = MODES_SHORT_MSG_BYTES
					case 16, 17, 18, 20, 21, 24:
						// keep MODES_LONG_MSG_BYTES
					default:
						bytelen = 1
					}
				}
			}

			score := messageScore(curMsg, bytelen*8)
			if score > bestScore {
				bestScore = score
				if useMsg1 {
					bestMsg = msg1
				} else {
					bestMsg = msg2
				}
				useMsg1 = !useMsg1
			}
		}

		if bestScore < 0 {
			continue
		}

		msglen := messageLenByType(bestMsg[0] >> 3)

		var sigSum uint64
		signalLen := msglen * 12 / 5
		for k := 0; k < signalLen && j+19+k < len(mag); k++ {
			m := uint32(mag[j+19+k])
			sigSum += uint64(m) * uint64(m)
		}
		sigLevel := float64(sigSum) / 65535.0 / 65535.0 / float64(signalLen)

		rm := RawMessage{
			Score:    bestScore,
			SigLevel: sigLevel,
			Bits:     msglen,
		}
		copy(rm.Data[:], bestMsg[:MODES_LONG_MSG_BYTES])
		results = append(results, rm)

		j += msglen * 12 / 5
	}

	return results
}

func bit(val int, pos uint) byte {
	if val > 0 {
		return 1 << pos
	}
	return 0
}

func messageLenByType(df byte) int {
	switch df {
	case 0, 4, 5, 11:
		return MODES_SHORT_MSG_BITS
	default:
		return MODES_LONG_MSG_BITS
	}
}
