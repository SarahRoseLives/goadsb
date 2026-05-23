package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"
)

var (
	flagFreq     = flag.Float64("freq", 1090.0, "Frequency in MHz")
	flagGain     = flag.Int("gain", 496, "Tuner gain in tenths of dB (0=auto, negative=AGC, max=496 for 49.6dB)")
	flagPPM      = flag.Int("ppm", 0, "Frequency correction in PPM")
	flagDevIdx   = flag.Int("device", 0, "RTL-SDR device index")
	flagRate     = flag.Float64("rate", 2.4, "Sample rate in MHz")
	flagBeast    = flag.Bool("beast", true, "Enable Beast binary output to stdout")
	flagBeastTCP = flag.String("beast-tcp", ":30005", "TCP address for Beast output (empty to disable)")
	flagQuiet    = flag.Bool("quiet", false, "Disable on-screen aircraft display")
	flagPhaseEnh = flag.Bool("phase-enhance", false, "Enable phase enhancement (slower, more sensitive)")
	flagFixBits  = flag.Int("fix", 1, "CRC error correction bits (0=none, 1=1-bit, 2=2-bit)")
)

func main() {
	flag.Parse()

	if *flagFixBits > 2 {
		*flagFixBits = 2
	}
	InitCRC(*flagFixBits)

	fmt.Fprintf(os.Stderr, "goadsb - ADS-B decoder for RTL-SDR\r\n")
	fmt.Fprintf(os.Stderr, "Frequency: %.1f MHz, Sample rate: %.1f MHz, Gain: %s, Fix: %d-bit\r\n",
		*flagFreq, *flagRate, gainStr(), *flagFixBits)

	dev, err := OpenRTL(*flagDevIdx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening RTL-SDR: %v\r\n", err)
		fmt.Fprintf(os.Stderr, "Make sure the RTL-SDR dongle is plugged in and drivers are installed.\r\n")
		os.Exit(1)
	}
	defer dev.Close()

	dev.SetCenterFreq(int(*flagFreq * 1e6))
	dev.SetSampleRate(int(*flagRate * 1e6))
	dev.SetFreqCorrection(*flagPPM)

	if *flagGain <= 0 {
		dev.SetAGCMode(true)
	} else {
		dev.SetAGCMode(false)
		dev.SetTunerGain(*flagGain)
	}
	dev.ResetBuffer()

	dc := &Decoder{
		phaseEnhance: *flagPhaseEnh,
		rateMHz:      *flagRate,
		display:      NewDisplay(!*flagQuiet),
	}

	if *flagBeastTCP != "" {
		dc.StartBeastServer(*flagBeastTCP)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	runtime.LockOSThread()

	go func() {
		<-sigCh
		fmt.Fprintf(os.Stderr, "\r\nShutting down...\r\n")
		dev.CancelAsync()
	}()

	callback := func(samples []byte) {
		dc.ProcessSamples(samples)
	}

	fmt.Fprintf(os.Stderr, "Listening... Press Ctrl+C to stop.\r\n\r\n")

	start := time.Now()

	err = dev.ReadAsync(callback)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ReadAsync error: %v\r\n", err)
	}

	dc.StopDisplay()
	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "\r\nRuntime: %v\r\n", elapsed)
	dc.PrintStats()
}

func gainStr() string {
	if *flagGain <= 0 {
		return "AGC"
	}
	return fmt.Sprintf("%d", *flagGain)
}
