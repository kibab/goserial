//go:build linux
// +build linux

package goserial

import (
	"log"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestConnection(t *testing.T) {
	// TODO : this could also be done using socat port-to-port emulation
	// that would make the test at least somewhat less system-specific
	port0 := os.Getenv("PORT0")
	port1 := os.Getenv("PORT1")
	if port0 == "" || port1 == "" {
		t.Skip("Skipping test because PORT0 or PORT1 environment variable is not set")
	}

	c0 := &Config{Name: port0, Baud: 115200}
	c1 := &Config{Name: port1, Baud: 115200}

	s1, err := OpenPort(c0)
	if err != nil {
		t.Fatal(err)
	}

	s2, err := OpenPort(c1)
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan int, 1)

	// FIXME SA2002
	// return the error on a channel
	go func() {
		buf := make([]byte, 128)
		var readCount int
		for {
			n, err := s2.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
			readCount++
			t.Logf("Read %v %v bytes: % 02x %s", readCount, n, buf[:n], buf[:n])
			select {
			case <-ch:
				ch <- readCount
				close(ch)
			default:
			}
		}
	}()

	if _, err = s1.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if _, err = s1.Write([]byte(" ")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	if _, err = s1.Write([]byte("world")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second / 10)

	ch <- 0
	_, err = s1.Write([]byte(" ")) // We could be blocked in the read without this
	if err != nil {
		t.Fatalf("error on write to serial port 1; %v", err)
	}
	c := <-ch
	exp := 5
	if c >= exp {
		t.Fatalf("Expected less than %v read, got %v", exp, c)
	}
}

func TestFindSerial(t *testing.T) {
	found, err := FindSerial()
	os := runtime.GOOS
	if err != nil && (os == "windows" || os == "darwin" || os == "linux") {
		t.Fatalf("error discovering serial ports; %v", err)
	}
	log.Println(found, err)
}
