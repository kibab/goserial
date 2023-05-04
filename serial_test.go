//go:build linux
// +build linux

package goserial

import (
	"context"
	"log"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestConnection(t *testing.T) {
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

func TestConnectionLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping socat test because not on Linux")
	}
	_, err := exec.LookPath("socat")
	if err != nil {
		t.Skip("Skipping test because socat was not found in PATH")
	}

	// timeout is a fallback here, if something fails
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	wd, _ := os.Getwd()
	cmd := "socat"
	args := []string{"-d", "-d", "pty,raw,echo=0,link=/tmp/pty0", "pty,raw,echo=0,link=/tmp/pty1"}

	err = startCmd(ctx, wd, cmd, args...)
	if err != nil {
		t.Fatal(err)
	}

	port0 := &Config{
		Name:        "/tmp/pty0",
		Baud:        9600,
		ReadTimeout: time.Duration(time.Second * 3),
		Size:        8,
	}
	port1 := &Config{
		Name:        "/tmp/pty1",
		Baud:        9600,
		ReadTimeout: time.Duration(time.Second * 3),
		Size:        8,
	}

	stream0, err := OpenPort(port0)
	if err != nil {
		t.Fatal("could not setup connection", err)
	}
	stream1, err := OpenPort(port1)
	if err != nil {
		t.Fatal("could not setup connection", err)
	}

	want := []byte("Hello, World!")
	nIn, err := stream1.Write(want)
	if err != nil {
		t.Fatal(err)
	}

	// stream is buffered, so we can read in sequence:
	buf := make([]byte, 1024)
	nOut, err := stream0.Read(buf)
	if err != nil {
		t.Fatal(err)
	}

	if nOut != nIn {
		t.Fatalf("sent %v bytes, got %v", nIn, nOut)
	}

	have := buf[:nOut]
	if !testEq(have, want) {
		t.Fatal("read data does not match written data")
	}

	cancel()
}

func TestFindSerial(t *testing.T) {
	found, err := FindSerial()
	os := runtime.GOOS
	if err != nil && (os == "windows" || os == "darwin" || os == "linux") {
		t.Fatalf("error discovering serial ports; %v", err)
	}
	log.Println(found, err)
}

func startCmd(ctx context.Context, wd, cmd string, args ...string) error {
	ecmd := exec.CommandContext(ctx, cmd, args...)
	ecmd.Dir = wd
	return ecmd.Start()
}

func testEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
