# goadsb

A pure Go ADS-B decoder for RTL-SDR dongles. Listens to 1090 MHz Mode S/ADS-B transmissions and outputs decoded aircraft data in Beast binary format.

Ported from the [dump1090](https://github.com/MalcolmRobb/dump1090) demodulator and decoder.

## Features

- **RTL-SDR support** via pure Go DLL binding (no CGo required)
- **2.4 MHz demodulator** based on Oliver Jowett's `demod-2400.c` algorithm
- **ADS-B decoding**: ICAO address, callsign, altitude, speed, heading, position (CPR), vertical rate, squawk
- **CRC checking** with 1-bit error correction
- **Beast binary output** on stdout (`*<hex>;\n` format) and TCP server
- **On-screen aircraft display** on stderr
- **Aircraft tracking** with Compact Position Reporting (CPR) decoding
- **Single binary** - no dependencies beyond the RTL-SDR driver DLL

## Requirements

- Windows (uses `syscall.NewLazyDLL` for DLL loading)
- RTL-SDR dongle with drivers installed (Zadig/WinUSB)
- `rtlsdr.dll` on PATH or in one of:
  - `C:\Program Files\SDRangel\rtlsdr.dll`
  - `C:\Program Files\SDR-Radio.com (V3)\rtlsdr.dll`
  - `C:\Windows\System32\rtlsdr.dll`

## Quick Start

```powershell
# Run with defaults (1090 MHz, 2.4 MS/s, AGC, 1-bit CRC fix)
.\goadsb.exe

# Custom settings
.\goadsb.exe -gain 496 -ppm 2 -freq 1090.0

# Quiet mode (no on-screen display, Beast output only)
.\goadsb.exe -quiet

# With TCP Beast server on custom port
.\goadsb.exe -beast-tcp ":30005"
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `--freq` | 1090.0 | Frequency in MHz |
| `--gain` | 496 | Tuner gain in tenths of dB (496=49.6dB, negative=AGC) |
| `--ppm` | 0 | Frequency correction in PPM |
| `--rate` | 2.4 | Sample rate in MHz |
| `--device` | 0 | RTL-SDR device index |
| `--beast` | true | Enable Beast binary output to stdout |
| `--beast-tcp` | :30005 | TCP address for Beast output (empty to disable) |
| `--quiet` | false | Disable on-screen aircraft display |
| `--fix` | 1 | CRC error correction bits (0=none, 1=1-bit, 2=2-bit) |
| `--phase-enhance` | false | Enable phase enhancement (slower, more sensitive) |

## Building

```powershell
go build -ldflags="-s -w" .
```

Requires Go 1.21+.

## Output Formats

### Beast Binary (stdout)
```
*8DA04641EA4C4864013C089A6425;
*8DA0464158C903ED06565F4315C6;
```

### On-Screen Display (stderr)
```
--- goadsb  Aircraft:2  Msgs:45  Good:32  Bad:13 ---
ICAO    Callsign     Alt    Spd       Lat       Lon  Hdg    V/s   Msgs
------- --------- ------ ------ --------- --------- ---- ------  -----
A04641  DAL1234    36250    450   40.7128  -74.0060  270    -64     25
A1B2C3  --------   12000    220   40.6500  -73.7800   90      0     12
```

## Architecture

```
RTL-SDR → rx callback → IQ→Mag LUT → Ring Buffer → Demodulator (2.4MHz)
  → Preamble Detect → Bit Slice → CRC Check → Message Decode
  → Aircraft Tracker (CPR) → Beast Output + Display
```

## License

MIT
