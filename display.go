package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

type BeastWriter struct {
	mu       sync.Mutex
	clients  map[net.Conn]bool
	listener net.Listener
	stdout   bool
}

func NewBeastWriter(tcpAddr string, stdout bool) *BeastWriter {
	bw := &BeastWriter{
		clients: make(map[net.Conn]bool),
		stdout:  stdout,
	}

	if tcpAddr != "" {
		ln, err := net.Listen("tcp", tcpAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Beast TCP listen on %s: %v\r\n", tcpAddr, err)
		} else {
			bw.listener = ln
			fmt.Fprintf(os.Stderr, "Beast output listening on %s\r\n", tcpAddr)
			go bw.acceptLoop()
		}
	}

	return bw
}

func (bw *BeastWriter) acceptLoop() {
	for {
		conn, err := bw.listener.Accept()
		if err != nil {
			return
		}
		bw.mu.Lock()
		bw.clients[conn] = true
		bw.mu.Unlock()
	}
}

func (bw *BeastWriter) Write(msg *DecodedMessage) {
	if !msg.CRC {
		return
	}

	line := fmt.Sprintf("*%s;\n", msg.Hex())

	if bw.stdout {
		os.Stdout.WriteString(line)
	}

	bw.mu.Lock()
	for conn := range bw.clients {
		conn.Write([]byte(line))
	}
	bw.mu.Unlock()
}

func (bw *BeastWriter) Close() {
	if bw.listener != nil {
		bw.listener.Close()
	}
	bw.mu.Lock()
	for conn := range bw.clients {
		conn.Close()
	}
	bw.mu.Unlock()
}

type Display struct {
	mu      sync.Mutex
	enabled bool
}

func NewDisplay(enabled bool) *Display {
	return &Display{enabled: enabled}
}

func (d *Display) IsEnabled() bool {
	return d.enabled
}

func (d *Display) Render(aircraft []*Aircraft, stats *DecoderStats) {
	if !d.enabled {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	io.WriteString(os.Stderr, d.formatScreen(aircraft, stats))
}

func (d *Display) formatScreen(aircraft []*Aircraft, stats *DecoderStats) string {
	out := fmt.Sprintf("\r\n--- goadsb  Aircraft:%d  Msgs:%d  Good:%d  Bad:%d ---\r\n",
		len(aircraft), stats.TotalMsgs, stats.GoodCRC, stats.BadCRC)
	out += fmt.Sprintf("%-7s %-9s %6s %6s %9s %9s %4s %6s  %5s\r\n",
		"ICAO", "Callsign", "Alt", "Spd", "Lat", "Lon", "Hdg", "V/s", "Msgs")
	out += "------- --------- ------ ------ --------- --------- ---- ------  -----\r\n"

	if len(aircraft) == 0 {
		out += "(no aircraft)\r\n"
	}

	for _, a := range aircraft {
		icao := fmt.Sprintf("%06X", a.ICAO)
		cs := a.Callsign
		if cs == "" {
			cs = "--------"
		}
		alt := ""
		if a.Altitude != 0 {
			alt = fmt.Sprintf("%d", a.Altitude)
		}
		spd := ""
		if a.Speed > 0 {
			spd = fmt.Sprintf("%.0f", a.Speed)
		}
		lat := ""
		lon := ""
		if a.HasPos {
			lat = fmt.Sprintf("%.4f", a.Lat)
			lon = fmt.Sprintf("%.4f", a.Lon)
		}
		hdg := ""
		if a.Heading > 0 {
			hdg = fmt.Sprintf("%.0f", a.Heading)
		}
		vs := ""
		if a.VertRate != 0 {
			vs = fmt.Sprintf("%d", a.VertRate)
		}

		out += fmt.Sprintf("%-7s %-9s %6s %6s %9s %9s %4s %6s  %5d\r\n",
			icao, cs, alt, spd, lat, lon, hdg, vs, a.MsgCount)
	}
	return out
}
