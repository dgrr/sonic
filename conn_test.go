package sonic

import (
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/talostrading/sonic/sonicopts"
)

func TestAsyncTCPEchoClient(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	closer := make(chan struct{}, 1)

	go func() {
		ln, err := net.Listen("tcp", "localhost:8080")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)
	outer:
		for {
			select {
			case <-closer:
				break outer
			default:
			}

			conn.Write([]byte("hello"))

			b = b[:cap(b)]
			n, err := conn.Read(b)
			if err != nil {
				panic(err)
			}

			if string(b[:n]) != "hello" {
				panic(fmt.Errorf("did not read %v", string(b)))
			}
		}
	}()

	conn, err := Dial(ioc, "tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}

	b := make([]byte, 5)
	var onAsyncRead AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
		} else {
			b = b[:n]
			if string(b) != "hello" {
				t.Fatalf("did not read %v", string(b))
			}

			conn.AsyncWriteAll(b, func(err error, n int) {
				if err != nil {
					if !errors.Is(err, io.EOF) || !errors.Is(err, syscall.EPIPE) {
						t.Fatal(err)
					}
				} else {
					b = b[:5]
					conn.AsyncReadAll(b, onAsyncRead)
				}
			})
		}
	}

	conn.AsyncReadAll(b, onAsyncRead)

	for i := 0; i < 10; i++ {
		ioc.RunOne()
	}

	closer <- struct{}{}
}

func TestAsyncTCPEchoServer(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	go func() {
		// Wait until there's a listener.
		time.Sleep(200 * time.Millisecond)

		conn, err := net.Dial("tcp", "localhost:8081")
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)

		for {
			conn.Write([]byte("hello"))
			n, err := conn.Read(b)
			if err != nil {
				panic(err)
			}

			b = b[:n]

			if string(b) != "hello" {
				panic(fmt.Errorf("did not read %v", string(b)))
			}
		}
	}()

	ln, err := Listen(ioc, "tcp", "localhost:8081", sonicopts.Nonblocking(true))
	if err != nil {
		t.Fatal(err)
	}

	handle := func(conn Conn) {
		b := make([]byte, 5)
		var onAsyncRead AsyncCallback
		onAsyncRead = func(err error, n int) {
			if err != nil {
				if err != io.EOF {
					t.Fatal(err)
				}
			} else {
				b = b[:n]

				if string(b) != "hello" {
					t.Fatalf("did not read %v", string(b))
				}

				conn.AsyncWriteAll(b, func(err error, n int) {
					if err != nil {
						if !errors.Is(err, io.EOF) || !errors.Is(err, syscall.EPIPE) {
							t.Fatal(err)
						}
					} else {
						b = b[:cap(b)]
						conn.AsyncReadAll(b, onAsyncRead)
					}
				})
			}
		}
		conn.AsyncReadAll(b, onAsyncRead)
	}

	ln.AsyncAccept(func(err error, conn Conn) {
		if err != nil {
			t.Fatalf("error on async accept err=%v", err)
		} else {
			handle(conn)
		}
	})

	for i := 0; i < 10; i++ {
		ioc.RunOne()
	}

	ln.Close()
}

func TestReadHandlesError(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	go func() {
		ln, err := net.Listen("tcp", "localhost:8082")
		if err != nil {
			panic(err)
		}

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		_, err = conn.Write([]byte("hello"))
		if err != nil {
			panic(err)
		}

		conn.Close()
	}()

	conn, err := Dial(ioc, "tcp", "localhost:8082")
	if err != nil {
		t.Fatal(err)
	}

	done := false
	b := make([]byte, 128)
	var onAsyncRead AsyncCallback
	onAsyncRead = func(err error, n int) {
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			} else {
				done = true
			}
		} else {
			b = b[:cap(b)]
			conn.AsyncReadAll(b, onAsyncRead)
		}
	}
	conn.AsyncReadAll(b, onAsyncRead)

	ioc.RunPending()

	if !done {
		t.Fatal("test did not run to completion")
	}
}

func TestWriteHandlesError(t *testing.T) {
	ioc := MustIO()
	defer ioc.Close()

	go func() {
		ln, err := net.Listen("tcp", "localhost:8083")
		if err != nil {
			panic(err)
		}
		defer ln.Close()

		conn, err := ln.Accept()
		if err != nil {
			panic(err)
		}

		b := make([]byte, 128)
		_, err = conn.Read(b)
		if err != nil {
			panic(err)
		}

		err = conn.Close()
		if err != nil {
			panic(err)
		}
	}()

	conn, err := Dial(ioc, "tcp", "localhost:8083")
	if err != nil {
		t.Fatal(err)
	}

	done := false
	var onAsyncWrite AsyncCallback
	onAsyncWrite = func(err error, n int) {
		if err != nil {
			if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
				done = true
			} else {
				t.Fatal(err)
			}
		} else {
			conn.AsyncWriteAll([]byte("hello"), onAsyncWrite)
		}
	}
	conn.AsyncWriteAll([]byte("hello"), onAsyncWrite)

	ioc.RunPending()

	if !done {
		t.Fatal("test did not run to completion")
	}
}