package main

import (
	"math"
	"sync"
	"time"
)

type Aircraft struct {
	ICAO      uint32
	Callsign  string
	Altitude  int32
	Speed     float64
	Heading   float64
	VertRate  int32
	Lat       float64
	Lon       float64
	HasPos    bool
	Squawk    uint16
	MsgCount  uint64
	LastSeen  time.Time
	FirstSeen time.Time
	SigLevel  float64

	cprOddLat   float64
	cprOddLon   float64
	cprEvenLat  float64
	cprEvenLon  float64
	cprOddTime  time.Time
	cprEvenTime time.Time
	cprValid    bool
}

type AircraftTracker struct {
	mu       sync.RWMutex
	aircraft map[uint32]*Aircraft
	homeLat  float64
	homeLon  float64
	ttl      time.Duration
}

func NewAircraftTracker(homeLat, homeLon float64) *AircraftTracker {
	return &AircraftTracker{
		aircraft: make(map[uint32]*Aircraft),
		homeLat:  homeLat,
		homeLon:  homeLon,
		ttl:      60 * time.Second,
	}
}

func (t *AircraftTracker) Update(msg *DecodedMessage) {
	t.mu.Lock()
	defer t.mu.Unlock()

	a, ok := t.aircraft[msg.ICAO]
	if !ok {
		a = &Aircraft{ICAO: msg.ICAO, FirstSeen: time.Now()}
		t.aircraft[msg.ICAO] = a
	}

	a.LastSeen = time.Now()
	a.MsgCount++
	a.SigLevel = msg.SigLevel

	if msg.Callsign != "" {
		a.Callsign = msg.Callsign
	}

	if msg.Altitude != 0 || msg.ME_Type >= 9 && msg.ME_Type <= 22 {
		a.Altitude = msg.Altitude
	}

	if msg.Speed > 0 {
		a.Speed = msg.Speed
	}
	if msg.Heading > 0 {
		a.Heading = msg.Heading
	}
	if msg.VertRate != 0 {
		a.VertRate = msg.VertRate
	}
	if msg.Squawk > 0 {
		a.Squawk = msg.Squawk
	}

	if msg.ME_Type >= 5 && msg.ME_Type <= 22 {
		updateCPR(a, msg)
	}
}

func updateCPR(a *Aircraft, msg *DecodedMessage) {
	if msg.CPREven {
		a.cprEvenLat = msg.Lat
		a.cprEvenLon = msg.Lon
		a.cprEvenTime = time.Now()
	} else {
		a.cprOddLat = msg.Lat
		a.cprOddLon = msg.Lon
		a.cprOddTime = time.Now()
	}

	if msg.CPREven && a.cprOddTime.IsZero() {
		return
	}
	if !msg.CPREven && a.cprEvenTime.IsZero() {
		return
	}

	cprEven := struct{ lat, lon float64 }{a.cprEvenLat, a.cprEvenLon}
	cprOdd := struct{ lat, lon float64 }{a.cprOddLat, a.cprOddLon}

	if !msg.CPREven {
		cprEven, cprOdd = cprOdd, cprEven
	}

	if msg.ME_Type >= 5 && msg.ME_Type <= 8 {
		a.Lat, a.Lon = surfaceCPRDecode(cprEven.lat, cprEven.lon, cprOdd.lat, cprOdd.lon)
	} else {
		a.Lat, a.Lon = airborneCPRDecode(cprEven.lat, cprEven.lon, cprOdd.lat, cprOdd.lon)
	}

	a.cprValid = true
	a.HasPos = true
}

func nl(declat float64) float64 {
	if declat >= 87.0 || declat <= -87.0 {
		return 1.0
	}
	coslat := math.Cos(declat * math.Pi / 180.0)
	return math.Floor((2.0 * math.Pi) / math.Acos(1.0 - (1.0-coslat)/(coslat*coslat)))
}

func airborneCPRDecode(evenLat, evenLon, oddLat, oddLon float64) (float64, float64) {
	dLatEven := 360.0 / 60.0
	dLatOdd := 360.0 / 59.0

	j := math.Floor(59.0*evenLat - 60.0*oddLat + 0.5)

	latEven := dLatEven * (float64(int(j)%60) + evenLat)
	latOdd := dLatOdd * (float64(int(j)%59) + oddLat)

	if latEven >= 270 {
		latEven -= 360
	}
	if latOdd >= 270 {
		latOdd -= 360
	}

	if nl(latEven) != nl(latOdd) {
		return latEven, evenLon
	}

	ni := nl(latEven)
	if ni < 1 {
		ni = 1
	}

	dLon := 360.0 / ni

	m := math.Floor(evenLon*(ni-1.0) - oddLon*ni + 0.5)
	lon := dLon * (float64(int(m)%int(ni)) + evenLon)
	if lon >= 180 {
		lon -= 360
	}

	return latEven, lon
}

func surfaceCPRDecode(evenLat, evenLon, oddLat, oddLon float64) (float64, float64) {
	dLatEven := 90.0 / 60.0
	dLatOdd := 90.0 / 59.0

	j := math.Floor(59.0*evenLat - 60.0*oddLat + 0.5)
	latEven := dLatEven * (float64(int(j)%60) + evenLat)
	latOdd := dLatOdd * (float64(int(j)%59) + oddLat)

	if latEven >= 270 {
		latEven -= 360
	}
	if latOdd >= 270 {
		latOdd -= 360
	}

	if nl(latEven) != nl(latOdd) {
		return latEven, evenLon
	}

	ni := nl(latEven)
	if ni < 1 {
		ni = 1
	}

	dLon := 90.0 / ni
	m := math.Floor(evenLon*(ni-1.0) - oddLon*ni + 0.5)
	lon := dLon * (float64(int(m)%int(ni)) + evenLon)
	if lon >= 180 {
		lon -= 360
	}

	return latEven, lon
}

func (t *AircraftTracker) RemoveStale() {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	for icao, a := range t.aircraft {
		if now.Sub(a.LastSeen) > t.ttl {
			delete(t.aircraft, icao)
		}
	}
}

func (t *AircraftTracker) List() []*Aircraft {
	t.mu.RLock()
	defer t.mu.RUnlock()

	list := make([]*Aircraft, 0, len(t.aircraft))
	for _, a := range t.aircraft {
		list = append(list, a)
	}
	return list
}

func (t *AircraftTracker) Count() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.aircraft)
}
