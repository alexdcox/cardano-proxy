package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
)

var nodeHostPort string
var proxyHostPort string
var proxyOutputMode string

func main() {
	flag.StringVar(&nodeHostPort, "node", "127.0.0.1:3031", "Cardano node host port")
	flag.StringVar(&proxyHostPort, "proxy", "127.0.0.1:3033", "Cardano proxy host port")
	flag.StringVar(&proxyOutputMode, "output", "write", "Cardano proxy output mode (dump|write)")
	flag.Parse()

	listen, err := net.Listen("tcp", proxyHostPort)
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}

	fmt.Printf("target node: %s\n", nodeHostPort)
	fmt.Printf("proxy:       %s\n", proxyHostPort)
	fmt.Printf("output mode: %s\n", proxyOutputMode)

	for {
		conn, err2 := listen.Accept()
		if err2 != nil {
			log.Fatalf("%+v", errors.WithStack(err2))
		}

		go handleClientConnection(conn)
	}
}

func handleClientConnection(client net.Conn) {
	fmt.Printf("new connection from %v\n", client.RemoteAddr())
	defer client.Close()

	dir := filepath.Join("./data/", fmt.Sprintf("proxy-%s", time.Now().Format("20060102-150405")))
	fmt.Printf("output dir:  %s\n\n", dir)
	err := os.MkdirAll(dir, os.ModePerm)

	requestIndex := new(atomic.Uint64)

	node, err := net.Dial("tcp", nodeHostPort)
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}

	go func() {
		fullBuf := new(bytes.Buffer)
		readBuf := make([]byte, 2^20*20) // 1MB
		for {
			n, err2 := node.Read(readBuf)
			if err2 != nil {
				if strings.Contains(err2.Error(), "EOF") {
					fmt.Println("node closed socket")
					_ = client.Close()
					return
				}
				if strings.Contains(err2.Error(), "connection reset") {
					fmt.Println("node reset connection")
					_ = client.Close()
					return
				}
				log.Fatalf("%+v", errors.WithStack(err2))
			}

			// fmt.Printf("node sent %d bytes\n", n)
			output(requestIndex, dir, ">", readBuf[:n])

			if n == 0 {
				log.Fatalf("not expecting 0 bytes read")
			}

			fullBuf.Write(readBuf[:n])

			if n == 402 {
				continue
			}

			_, err = client.Write(fullBuf.Bytes())
			if err != nil {
				log.Printf("%+v", errors.WithStack(err))
				return
			}

			fullBuf = new(bytes.Buffer)
		}
	}()

	buf := make([]byte, 2^20*20)
	for {
		n, err3 := client.Read(buf)
		if err3 != nil {
			if strings.Contains(err3.Error(), "use of closed network connection") {
				return
			}
			if strings.Contains(err3.Error(), "EOF") {
				return
			}
			log.Printf("%+v", errors.WithStack(err3))
			return
		}

		output(requestIndex, dir, "<", buf[:n])
		_, err = node.Write(buf[:n])
		if err != nil {
			log.Printf("%+v", errors.WithStack(err))
			return
		}
	}
}

func output(requestIndex *atomic.Uint64, dir, direction string, buf []byte) {
	index := requestIndex.Add(1)
	if strings.Contains(proxyOutputMode, "write") {
		writeHex(index, dir, direction, buf)
	}
	if strings.Contains(proxyOutputMode, "dump") {
		dumpHex(index, direction, buf)
	}
}

func dumpHex(i uint64, direction string, buf []byte) {
	d := "OUT"
	if direction == ">" {
		d = "IN"
	}
	fmt.Printf("%08d | %s | %d bytes\n%x\n\n", i, d, len(buf), buf)
}

func writeHex(i uint64, dir, direction string, buf []byte) {
	d := "o"
	if direction == ">" {
		d = "i"
	}
	filename := fmt.Sprintf("%08d-%s-%d.dat", i, d, len(buf))
	fpath := path.Join(dir, filename)
	err := os.WriteFile(fpath, []byte(fmt.Sprintf("%x", buf)), os.ModePerm)
	// fmt.Printf("wrote: %s\n", filename)
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}
}
