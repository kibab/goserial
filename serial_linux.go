//go:build linux
// +build linux

package goserial

import (
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

func openPort(name string, baud int, databits byte, parity Parity, stopbits StopBits, readTimeout time.Duration) (*Port, error) {

	f, err := os.OpenFile(name, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0666)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil && f != nil {
			f.Close()
		}
	}()

	p := &Port{
		f:           f,
		baud:        baud,
		databits:    databits,
		parity:      parity,
		stopbits:    stopbits,
		readTimeout: readTimeout,
	}

	if err = p.setParams(); err != nil {
		return nil, err
	}

	if err = unix.SetNonblock(int(p.f.Fd()), false); err != nil {
		return nil, err
	}

	return p, nil
}

type Port struct {
	// We intentionally do not use an "embedded" struct so that we don't export File
	f *os.File

	baud        int
	databits    byte
	parity      Parity
	stopbits    StopBits
	readTimeout time.Duration
}

func (p *Port) Read(b []byte) (n int, err error) {
	return p.f.Read(b)
}

func (p *Port) Write(b []byte) (n int, err error) {
	return p.f.Write(b)
}

// Discards data written to the port but not transmitted, or data received but not read
func (p *Port) Flush() error {
	const TCFLSH = 0x540B
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(p.f.Fd()),
		uintptr(TCFLSH),
		uintptr(unix.TCIOFLUSH),
	)

	if errno == 0 {
		return nil
	}
	return errno
}

func (p *Port) Close() (err error) {
	return p.f.Close()
}

func (p *Port) SetSpeed(baud int) error {
	p.baud = baud
	return p.setParams()
}

func (p *Port) setParams() error {
	bauds := map[int]uint32{
		50:      unix.B50,
		75:      unix.B75,
		110:     unix.B110,
		134:     unix.B134,
		150:     unix.B150,
		200:     unix.B200,
		300:     unix.B300,
		600:     unix.B600,
		1200:    unix.B1200,
		1800:    unix.B1800,
		2400:    unix.B2400,
		4800:    unix.B4800,
		9600:    unix.B9600,
		19200:   unix.B19200,
		38400:   unix.B38400,
		57600:   unix.B57600,
		115200:  unix.B115200,
		230400:  unix.B230400,
		460800:  unix.B460800,
		500000:  unix.B500000,
		576000:  unix.B576000,
		921600:  unix.B921600,
		1000000: unix.B1000000,
		1152000: unix.B1152000,
		1500000: unix.B1500000,
		2000000: unix.B2000000,
		2500000: unix.B2500000,
		3000000: unix.B3000000,
		3500000: unix.B3500000,
		4000000: unix.B4000000,
	}

	rate, ok := bauds[p.baud]

	if !ok {
		return fmt.Errorf("unrecognized baud rate")
	}

	// Base settings
	cflagToUse := unix.CREAD | unix.CLOCAL | rate
	switch p.databits {
	case 5:
		cflagToUse |= unix.CS5
	case 6:
		cflagToUse |= unix.CS6
	case 7:
		cflagToUse |= unix.CS7
	case 8:
		cflagToUse |= unix.CS8
	default:
		return ErrBadSize
	}
	// Stop bits settings
	switch p.stopbits {
	case Stop1:
		// default is 1 stop bit
	case Stop2:
		cflagToUse |= unix.CSTOPB
	default:
		// Don't know how to set 1.5
		return ErrBadStopBits
	}
	// Parity settings
	switch p.parity {
	case ParityNone:
		// default is no parity
	case ParityOdd:
		cflagToUse |= unix.PARENB
		cflagToUse |= unix.PARODD
	case ParityEven:
		cflagToUse |= unix.PARENB
	default:
		return ErrBadParity
	}
	fd := p.f.Fd()
	vmin, vtime := posixTimeoutValues(p.readTimeout)
	t := unix.Termios{
		Iflag:  unix.IGNPAR,
		Cflag:  cflagToUse,
		Ispeed: rate,
		Ospeed: rate,
	}
	t.Cc[unix.VMIN] = vmin
	t.Cc[unix.VTIME] = vtime

	if _, _, errno := unix.Syscall6(
		unix.SYS_IOCTL,
		uintptr(fd),
		uintptr(unix.TCSETS),
		uintptr(unsafe.Pointer(&t)),
		0,
		0,
		0,
	); errno != 0 {
		return fmt.Errorf("cannot call IOCTL TCSETS: errno %d", errno)
	}

	return nil
}
