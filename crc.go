package main

const MODES_GENERATOR_POLY uint32 = 0xFFF409

var crcTable [256]uint32
var singleBitSyndrome [MODES_LONG_MSG_BITS]uint32

func init() {
	for i := 0; i < 256; i++ {
		c := uint32(i) << 16
		for j := 0; j < 8; j++ {
			if c&0x800000 != 0 {
				c = (c << 1) ^ MODES_GENERATOR_POLY
			} else {
				c = c << 1
			}
		}
		crcTable[i] = c & 0x00FFFFFF
	}

	msg := make([]byte, MODES_LONG_MSG_BITS/8)
	for i := 0; i < MODES_LONG_MSG_BITS; i++ {
		msg[i/8] ^= 1 << (7 - (i & 7))
		singleBitSyndrome[i] = crcChecksum(msg, MODES_LONG_MSG_BITS)
		msg[i/8] ^= 1 << (7 - (i & 7))
	}
}

func InitCRC(fixBits int) {
	initErrorTables(fixBits)
}

func crcChecksum(msg []byte, bits int) uint32 {
	n := bits / 8
	rem := uint32(0)

	for i := 0; i < n-3; i++ {
		rem = (rem << 8) ^ crcTable[msg[i]^byte((rem&0xFF0000)>>16)]
		rem &= 0xFFFFFF
	}
	rem = rem ^ (uint32(msg[n-3]) << 16) ^ (uint32(msg[n-2]) << 8) ^ uint32(msg[n-1])
	return rem
}

type ErrorInfo struct {
	Syndrome uint32
	Errors   int
	Bits     [2]int
}

var NO_ERRORS = ErrorInfo{}

var shortErrors []ErrorInfo
var longErrors []ErrorInfo

func initErrorTables(fixBits int) {
	if fixBits == 0 {
		return
	}

	shortErrors = prepareErrorTable(MODES_SHORT_MSG_BITS, fixBits, fixBits)
	longErrors = prepareErrorTable(MODES_LONG_MSG_BITS, fixBits, fixBits)
}

func prepareErrorTable(bits, maxCorrect, maxDetect int) []ErrorInfo {
	if maxCorrect == 0 {
		return nil
	}

	maxSize := 0
	for i := 1; i <= maxCorrect; i++ {
		maxSize += combinations(bits-5, i)
	}

	table := make([]ErrorInfo, maxSize)
	entry := ErrorInfo{Bits: [2]int{-1, -1}}

	used := prepareSubTable(table, 0, maxSize, MODES_LONG_MSG_BITS-bits, 5, bits, &entry, 0, maxCorrect)

	table = table[:used]

	for i := 0; i < len(table); i++ {
		for j := i + 1; j < len(table); j++ {
			if table[j].Syndrome < table[i].Syndrome {
				table[i], table[j] = table[j], table[i]
			}
		}
	}

	dst := 0
	for i := 0; i < used; i++ {
		if i < used-1 && table[i+1].Syndrome == table[i].Syndrome {
			for i < used-1 && table[i+1].Syndrome == table[i].Syndrome {
				i++
			}
			continue
		}
		if dst != i {
			table[dst] = table[i]
		}
		dst++
	}
	table = table[:dst]

	if maxDetect > maxCorrect {
		flagged := flagCollisions(table, MODES_LONG_MSG_BITS-bits, 5, bits, 0, 1, maxCorrect+1, maxDetect)
		if flagged > 0 {
			dst = 0
			for i := 0; i < len(table); i++ {
				if table[i].Errors != -1 {
					if dst != i {
						table[dst] = table[i]
					}
					dst++
				}
			}
			table = table[:dst]
		}
	}

	return table
}

func prepareSubTable(table []ErrorInfo, n, maxSize, offset, startBit, endBit int, base *ErrorInfo, errorBit, maxErrors int) int {
	if errorBit >= maxErrors {
		return n
	}

	for i := startBit; i < endBit; i++ {
		table[n] = *base
		table[n].Syndrome ^= singleBitSyndrome[i+offset]
		table[n].Errors = errorBit + 1
		table[n].Bits[errorBit] = i
		n++
		n = prepareSubTable(table, n, maxSize, offset, i+1, endBit, &table[n-1], errorBit+1, maxErrors)
	}
	return n
}

func flagCollisions(table []ErrorInfo, offset, startBit, endBit int, baseSyndrome uint32, errorBit, firstError, lastError int) int {
	count := 0
	if errorBit > lastError {
		return 0
	}

	for i := startBit; i < endBit; i++ {
		ei := ErrorInfo{Syndrome: baseSyndrome ^ singleBitSyndrome[i+offset]}
		if errorBit >= firstError {
			if idx := searchErrorInfo(table, ei.Syndrome); idx >= 0 && table[idx].Errors != -1 {
				count++
				table[idx].Errors = -1
			}
		}
		count += flagCollisions(table, offset, i+1, endBit, ei.Syndrome, errorBit+1, firstError, lastError)
	}
	return count
}

func searchErrorInfo(table []ErrorInfo, syndrome uint32) int {
	lo, hi := 0, len(table)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if table[mid].Syndrome < syndrome {
			lo = mid + 1
		} else if table[mid].Syndrome > syndrome {
			hi = mid - 1
		} else {
			return mid
		}
	}
	return -1
}

func crcDiagnose(syndrome uint32, bitlen int) *ErrorInfo {
	if syndrome == 0 {
		return &NO_ERRORS
	}

	var table []ErrorInfo
	if bitlen == MODES_SHORT_MSG_BITS {
		table = shortErrors
	} else {
		table = longErrors
	}

	if table == nil {
		return nil
	}

	if idx := searchErrorInfo(table, syndrome); idx >= 0 {
		return &table[idx]
	}
	return nil
}

func crcFix(msg []byte, info *ErrorInfo) {
	for i := 0; i < info.Errors; i++ {
		msg[info.Bits[i]>>3] ^= 1 << (7 - (info.Bits[i]&7))
	}
}

func combinations(n, k int) int {
	if k == 0 || k == n {
		return 1
	}
	if k > n {
		return 0
	}
	result := 1
	for i := 1; i <= k; i++ {
		result = result * n / i
		n--
	}
	return result
}
