package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type DecoderStats struct {
	TotalMsgs uint64
	GoodCRC   uint64
	BadCRC    uint64
	Preamble  uint64
}

type Decoder struct {
	ring         *MagRing
	tracker      *AircraftTracker
	beast        *BeastWriter
	display      *Display
	phaseEnhance bool
	rateMHz      float64
	stats        DecoderStats

	stopCh chan struct{}
	wg     sync.WaitGroup
}

func (dc *Decoder) StartBeastServer(addr string) {
	dc.beast = NewBeastWriter(addr, *flagBeast)
}

func (dc *Decoder) ProcessSamples(samples []byte) {
	if dc.ring == nil {
		dc.ring = NewMagRing()
		dc.tracker = NewAircraftTracker(0, 0)
		dc.stopCh = make(chan struct{})

		dc.wg.Add(1)
		go dc.processLoop()
	}

	dc.ring.Write(samples)
}

func (dc *Decoder) processLoop() {
	defer dc.wg.Done()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	housekeep := time.NewTicker(250 * time.Millisecond)
	defer housekeep.Stop()

	for {
		select {
		case <-dc.stopCh:
			return
		case <-ticker.C:
			magData, readable, _ := dc.ring.CopySafe()
			if readable <= 0 {
				continue
			}

			results := demodulate2400(magData, readable, dc.phaseEnhance)

			for _, raw := range results {
				atomic.AddUint64(&dc.stats.TotalMsgs, 1)

				msg := decodeMessage(&raw)
				if msg.CRC {
					atomic.AddUint64(&dc.stats.GoodCRC, 1)

					if dc.beast != nil {
						dc.beast.Write(msg)
					}

					if dc.tracker != nil {
						dc.tracker.Update(msg)
					}
				} else {
					atomic.AddUint64(&dc.stats.BadCRC, 1)
				}
			}

		case <-housekeep.C:
			if dc.tracker != nil {
				dc.tracker.RemoveStale()
			}
			if dc.display.IsEnabled() {
				aircraft := dc.tracker.List()
				dc.display.Render(aircraft, &dc.stats)
			}
		}
	}
}

func (dc *Decoder) StopDisplay() {
	if dc.stopCh != nil {
		close(dc.stopCh)
	}
	dc.wg.Wait()

	if dc.beast != nil {
		dc.beast.Close()
	}

	fmt.Fprintf(os.Stderr, "\r\n")
}

func (dc *Decoder) PrintStats() {
	fmt.Fprintf(os.Stderr, "Messages: %d total, %d good CRC, %d bad CRC\r\n",
		dc.stats.TotalMsgs, dc.stats.GoodCRC, dc.stats.BadCRC)
	if dc.tracker != nil {
		fmt.Fprintf(os.Stderr, "Aircraft tracked: %d\r\n", dc.tracker.Count())
	}
}
