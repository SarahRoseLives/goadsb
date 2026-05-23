package main

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	rtlsdrDLL *syscall.LazyDLL

	rtlsdrGetDeviceCount    *syscall.LazyProc
	rtlsdrGetDeviceName     *syscall.LazyProc
	rtlsdrOpen              *syscall.LazyProc
	rtlsdrClose             *syscall.LazyProc
	rtlsdrSetCenterFreq     *syscall.LazyProc
	rtlsdrSetSampleRate     *syscall.LazyProc
	rtlsdrSetTunerGainMode  *syscall.LazyProc
	rtlsdrSetTunerGain      *syscall.LazyProc
	rtlsdrSetAGCMode        *syscall.LazyProc
	rtlsdrSetFreqCorrection *syscall.LazyProc
	rtlsdrResetBuffer       *syscall.LazyProc
	rtlsdrReadAsync         *syscall.LazyProc
	rtlsdrCancelAsync       *syscall.LazyProc
)

type RTLDevice struct {
	dev    uintptr
	cancel chan struct{}
}

func initRTL() error {
	if rtlsdrDLL != nil {
		return nil
	}

	paths := []string{
		"rtlsdr.dll",
		"C:\\Program Files\\SDRangel\\rtlsdr.dll",
		"C:\\Program Files\\SDR-Radio.com (V3)\\rtlsdr.dll",
		"C:\\Windows\\System32\\rtlsdr.dll",
	}

	var lastErr error
	for _, p := range paths {
		dll := syscall.NewLazyDLL(p)
		err := dll.Load()
		if err == nil {
			rtlsdrDLL = dll
			break
		}
		lastErr = err
	}

	if rtlsdrDLL == nil {
		return fmt.Errorf("failed to load rtlsdr.dll: %v", lastErr)
	}

	rtlsdrGetDeviceCount = rtlsdrDLL.NewProc("rtlsdr_get_device_count")
	rtlsdrGetDeviceName = rtlsdrDLL.NewProc("rtlsdr_get_device_name")
	rtlsdrOpen = rtlsdrDLL.NewProc("rtlsdr_open")
	rtlsdrClose = rtlsdrDLL.NewProc("rtlsdr_close")
	rtlsdrSetCenterFreq = rtlsdrDLL.NewProc("rtlsdr_set_center_freq")
	rtlsdrSetSampleRate = rtlsdrDLL.NewProc("rtlsdr_set_sample_rate")
	rtlsdrSetTunerGainMode = rtlsdrDLL.NewProc("rtlsdr_set_tuner_gain_mode")
	rtlsdrSetTunerGain = rtlsdrDLL.NewProc("rtlsdr_set_tuner_gain")
	rtlsdrSetAGCMode = rtlsdrDLL.NewProc("rtlsdr_set_agc_mode")
	rtlsdrSetFreqCorrection = rtlsdrDLL.NewProc("rtlsdr_set_freq_correction")
	rtlsdrResetBuffer = rtlsdrDLL.NewProc("rtlsdr_reset_buffer")
	rtlsdrReadAsync = rtlsdrDLL.NewProc("rtlsdr_read_async")
	rtlsdrCancelAsync = rtlsdrDLL.NewProc("rtlsdr_cancel_async")

	return nil
}

type rtlsdrReadAsyncCb func(buf *byte, length uint32, ctx unsafe.Pointer)

func OpenRTL(index int) (*RTLDevice, error) {
	if err := initRTL(); err != nil {
		return nil, err
	}

	count, _, _ := rtlsdrGetDeviceCount.Call()
	if count == 0 {
		return nil, fmt.Errorf("no RTL-SDR devices found")
	}
	if index >= int(count) {
		return nil, fmt.Errorf("device index %d out of range (found %d device(s))", index, int(count))
	}

	var dev uintptr
	ret, _, _ := rtlsdrOpen.Call(uintptr(unsafe.Pointer(&dev)), uintptr(index))
	if ret != 0 {
		return nil, fmt.Errorf("rtlsdr_open failed with code %d", int(ret))
	}

	nameBuf := make([]byte, 256)
	rtlsdrGetDeviceName.Call(uintptr(index), uintptr(unsafe.Pointer(&nameBuf[0])), uintptr(len(nameBuf)))
	name := ""
	for _, b := range nameBuf {
		if b == 0 {
			break
		}
		name += string(b)
	}

	fmt.Fprintf(os.Stderr, "Opened RTL-SDR device #%d: %s\r\n", index, name)
	return &RTLDevice{dev: dev, cancel: make(chan struct{})}, nil
}

func (d *RTLDevice) Close() {
	if d.dev != 0 {
		rtlsdrClose.Call(d.dev)
		d.dev = 0
	}
}

func (d *RTLDevice) SetCenterFreq(hz int) {
	if d.dev != 0 {
		rtlsdrSetCenterFreq.Call(d.dev, uintptr(hz))
	}
}

func (d *RTLDevice) SetSampleRate(hz int) {
	if d.dev != 0 {
		rtlsdrSetSampleRate.Call(d.dev, uintptr(hz))
	}
}

func (d *RTLDevice) SetTunerGain(gain int) {
	if d.dev != 0 {
		rtlsdrSetTunerGainMode.Call(d.dev, 1)
		rtlsdrSetTunerGain.Call(d.dev, uintptr(gain))
	}
}

func (d *RTLDevice) SetAGCMode(on bool) {
	if d.dev != 0 {
		val := uintptr(0)
		if on {
			val = 1
		}
		rtlsdrSetAGCMode.Call(d.dev, val)
	}
}

func (d *RTLDevice) SetFreqCorrection(ppm int) {
	if d.dev != 0 {
		rtlsdrSetFreqCorrection.Call(d.dev, uintptr(ppm))
	}
}

func (d *RTLDevice) ResetBuffer() {
	if d.dev != 0 {
		rtlsdrResetBuffer.Call(d.dev)
	}
}

func (d *RTLDevice) CancelAsync() {
	if d.dev != 0 {
		rtlsdrCancelAsync.Call(d.dev)
		d.cancel <- struct{}{}
	}
}

func (d *RTLDevice) ReadAsync(callback func([]byte)) error {
	if d.dev == 0 {
		return fmt.Errorf("device not open")
	}

	cbid := callbackCtx.add(callback)
	defer callbackCtx.remove(cbid)

	runtime.LockOSThread()

	cb := syscall.NewCallback(func(buf *byte, length uint32, ctx unsafe.Pointer) uintptr {
		data := unsafe.Slice(buf, int(length))
		callbackCtx.invoke(cbid, data)
		return 0
	})

	ret, _, _ := rtlsdrReadAsync.Call(d.dev, cb, uintptr(cbid), 0, 0)
	if ret != 0 {
		return fmt.Errorf("rtlsdr_read_async failed with code %d", int(ret))
	}

	<-d.cancel
	return nil
}

type callbackRegistry struct {
	cbs  map[uintptr]func([]byte)
	next uintptr
}

var callbackCtx = &callbackRegistry{cbs: make(map[uintptr]func([]byte))}

func (r *callbackRegistry) add(fn func([]byte)) uintptr {
	r.next++
	r.cbs[r.next] = fn
	return r.next
}

func (r *callbackRegistry) remove(id uintptr) {
	delete(r.cbs, id)
}

func (r *callbackRegistry) invoke(id uintptr, data []byte) {
	if fn, ok := r.cbs[id]; ok {
		fn(data)
	}
}
