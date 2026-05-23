package main

import (
	"encoding/hex"
	"math"
)

func messageScore(msg []byte, bits int) int {
	if bits < MODES_SHORT_MSG_BITS {
		return -1
	}

	df := msg[0] >> 3
	validDF := false

	switch df {
	case 0, 4, 5, 11:
		if bits == MODES_SHORT_MSG_BITS {
			validDF = true
		}
	case 16, 17, 18, 20, 21, 22, 24:
		if bits == MODES_LONG_MSG_BITS {
			validDF = true
		}
	}

	if !validDF {
		return -2
	}

	syndrome := crcChecksum(msg, bits)
	if syndrome == 0 {
		return 1000
	}

	info := crcDiagnose(syndrome, bits)
	if info != nil && info.Errors > 0 {
		return 350 + 150*info.Errors
	}

	return -1
}

type DecodedMessage struct {
	DF       byte
	CA       byte
	ICAO     uint32
	Data     []byte
	TC       byte
	ME       []byte
	CRC      bool
	Bits     int
	SigLevel float64

	ME_Type      byte
	ME_SubType   byte
	Altitude     int32
	AltitudeType string
	Lat          float64
	Lon          float64
	CPRType      byte
	CPREven      bool
	Speed        float64
	Heading      float64
	VertRate     int32
	Callsign     string
	Squawk       uint16
}

func decodeMessage(raw *RawMessage) *DecodedMessage {
	msg := make([]byte, MODES_LONG_MSG_BYTES)
	copy(msg, raw.Data[:])

	dm := &DecodedMessage{
		Data:     msg,
		Bits:     raw.Bits,
		SigLevel: raw.SigLevel,
	}

	dm.DF = msg[0] >> 3

	if dm.Bits == MODES_SHORT_MSG_BITS {
		dm.CA = msg[0] & 0x07
		dm.ICAO = (uint32(msg[1]) << 16) | (uint32(msg[2]) << 8) | uint32(msg[3])
		if dm.DF == 11 {
			dm.Squawk = ((uint16(msg[4]) & 0x7F) << 6) | (uint16(msg[5]) >> 2)
		}
		return dm
	}

	dm.CA = msg[0] & 0x07
	dm.ICAO = (uint32(msg[1]) << 16) | (uint32(msg[2]) << 8) | uint32(msg[3])

	syndrome := crcChecksum(msg, raw.Bits)
	if syndrome == 0 {
		dm.CRC = true
	} else {
		info := crcDiagnose(syndrome, raw.Bits)
		if info != nil && info.Errors > 0 {
			crcFix(msg, info)
			dm.ICAO = (uint32(msg[1]) << 16) | (uint32(msg[2]) << 8) | uint32(msg[3])
			dm.CRC = true
		} else {
			return dm
		}
	}

	dm.ME = msg[4:11]

	if dm.DF == 17 || dm.DF == 18 {
		decodeExtendedSquitter(dm)
	}

	return dm
}

func decodeExtendedSquitter(dm *DecodedMessage) {
	dm.ME_Type = dm.ME[0] >> 3
	dm.ME_SubType = dm.ME[0] & 0x07

	switch {
	case dm.ME_Type >= 1 && dm.ME_Type <= 4:
		decodeAircraftID(dm)
	case dm.ME_Type >= 5 && dm.ME_Type <= 8:
		decodeSurfacePosition(dm)
	case dm.ME_Type >= 9 && dm.ME_Type <= 18:
		decodeAirbornePosition(dm, false)
	case dm.ME_Type == 19:
		decodeAirborneVelocity(dm)
	case dm.ME_Type >= 20 && dm.ME_Type <= 22:
		decodeAirbornePosition(dm, true)
	}
}

func decodeAircraftID(dm *DecodedMessage) {
	charset := "?ABCDEFGHIJKLMNOPQRSTUVWXYZ????? ???????????????0123456789??????"
	chars1 := (uint32(dm.ME[1]) << 16) | (uint32(dm.ME[2]) << 8) | uint32(dm.ME[3])
	chars2 := (uint32(dm.ME[4]) << 16) | (uint32(dm.ME[5]) << 8) | uint32(dm.ME[6])

	dm.Callsign = ""
	for i := 0; i < 8; i++ {
		var ch uint32
		if i < 4 {
			ch = (chars1 >> uint((3-i)*6)) & 0x3F
		} else {
			ch = (chars2 >> uint((7-i)*6)) & 0x3F
		}
		if int(ch) < len(charset) {
			dm.Callsign += string(charset[ch])
		}
	}
}

func decodeAirbornePosition(dm *DecodedMessage, gnss bool) {
	dm.CPREven = (dm.ME[6]>>2)&1 == 0

	cprLat := uint32(dm.ME[0]&0x07)<<14 | uint32(dm.ME[1])<<6 | uint32(dm.ME[2])>>2
	cprLon := uint32(dm.ME[2]&0x03)<<15 | uint32(dm.ME[3])<<7 | uint32(dm.ME[4])>>1

	var alt int32
	if gnss {
		alt = decodeGNSSAltitude(dm.ME)
		dm.AltitudeType = "GNSS"
	} else {
		alt = decodeBaroAltitude(dm.ME)
		dm.AltitudeType = "BARO"
	}
	dm.Altitude = alt

	dm.CPRType = dm.ME[6] >> 2

	dm.Lat = float64(cprLat) / 131072.0
	dm.Lon = float64(cprLon) / 131072.0
}

func decodeBaroAltitude(me []byte) int32 {
	n := (uint32(me[2]&0x30) >> 2) | (uint32(me[3]) << 3) | (uint32(me[4]&0xC0) >> 6)
	if me[4]&0x20 != 0 {
		return int32(n)*25 - 1000
	}
	return int32(n)*100 - 1000
}

func decodeGNSSAltitude(me []byte) int32 {
	n := (uint32(me[4]&0x0F) << 6) | (uint32(me[5]) >> 2)
	if me[4]&0x10 != 0 {
		return int32(n) * 25
	}
	return int32(n) * 100
}

func decodeSurfacePosition(dm *DecodedMessage) {
	cprLat := uint32(dm.ME[0]&0x07)<<14 | uint32(dm.ME[1])<<6 | uint32(dm.ME[2])>>2
	cprLon := uint32(dm.ME[2]&0x03)<<15 | uint32(dm.ME[3])<<7 | uint32(dm.ME[4])>>1

	mov := dm.ME[4] & 0x07
	var speed float64
	switch mov {
	case 1:
		speed = 0
	case 2:
		speed = 0.125
	case 3:
		speed = 0.25
	case 4:
		speed = 0.5
	case 5:
		speed = 1.0
	case 6:
		speed = 2.0
	case 7:
		speed = 4.0
	}
	dm.Speed = speed

	dm.Heading = float64(((uint32(dm.ME[5]&0x07)<<4)|(uint32(dm.ME[6])>>4))*45) / 8.0

	dm.CPREven = (dm.ME[6]>>2)&1 == 0
	dm.Lat = float64(cprLat) / 131072.0
	dm.Lon = float64(cprLon) / 131072.0
}

func decodeAirborneVelocity(dm *DecodedMessage) {
	subtype := dm.ME[0] & 0x07

	if subtype == 1 || subtype == 2 {
		decodeVelocitySubsonic(dm, subtype)
	} else if subtype == 3 || subtype == 4 {
		decodeVelocitySupersonic(dm, subtype)
	}
}

func decodeVelocitySubsonic(dm *DecodedMessage, subtype byte) {
	ewRaw := (uint32(dm.ME[1]&0x03) << 8) | uint32(dm.ME[2])
	nsRaw := (uint32(dm.ME[3]&0x7F) << 3) | (uint32(dm.ME[4]) >> 5)

	var ewv, nsv float64
	if ewRaw > 0 {
		ewv = float64(ewRaw - 1)
		if dm.ME[1]&0x04 != 0 {
			ewv = -ewv
		}
	}
	if nsRaw > 0 {
		nsv = float64(nsRaw - 1)
		if dm.ME[3]&0x80 != 0 {
			nsv = -nsv
		}
	}

	if subtype == 2 {
		ewv *= 4
		nsv *= 4
	}

	if ewRaw > 0 || nsRaw > 0 {
		dm.Speed = math.Sqrt(ewv*ewv + nsv*nsv)
		dm.Heading = math.Atan2(ewv, nsv) * 180.0 / math.Pi
		if dm.Heading < 0 {
			dm.Heading += 360
		}
	}

	vr := ((uint32(dm.ME[4]&0x07) << 6) | (uint32(dm.ME[5]) >> 2))
	if vr > 0 {
		vr--
		if dm.ME[4]&0x08 != 0 {
			dm.VertRate = -int32(vr-1) * 64
		} else {
			dm.VertRate = int32(vr-1) * 64
		}
	}
}

func decodeVelocitySupersonic(dm *DecodedMessage, subtype byte) {
	airSpeed := (uint32(dm.ME[3]&0x7F) << 3) | (uint32(dm.ME[4]) >> 5)
	if airSpeed > 0 {
		airSpeed--
		if subtype == 4 {
			airSpeed = airSpeed << 2
		}
		dm.Speed = float64(airSpeed)
	}
	if dm.ME[1]&0x04 != 0 {
		dm.Heading = float64(((uint32(dm.ME[1]&0x03)<<8)|uint32(dm.ME[2]))*45) / 128.0
	}
}

func (dm *DecodedMessage) Hex() string {
	n := dm.Bits / 8
	return hex.EncodeToString(dm.Data[:n])
}
