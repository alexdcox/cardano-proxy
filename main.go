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

	"github.com/fxamacker/cbor/v2"
	"github.com/pkg/errors"
)

var dir string
var nodeHostPort string
var proxyHostPort string

func main() {
	flag.StringVar(&nodeHostPort, "node", "127.0.0.1:7731", "Cardano node host port")
	flag.StringVar(&proxyHostPort, "proxy", "127.0.0.1:7732", "Cardano proxy host port")
	flag.Parse()

	fmt.Printf("target node: %s\n", nodeHostPort)

	dir = filepath.Join(".", fmt.Sprintf("proxy-%s", time.Now().Format("20060102-150405")))
	err := os.MkdirAll(dir, os.ModePerm)

	listen, err := net.Listen("tcp", proxyHostPort)
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}

	fmt.Printf("proxy listening on %s\n", proxyHostPort)

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

	node, err := net.Dial("tcp", nodeHostPort)
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}

	go func() {
		fullBuf := new(bytes.Buffer)
		readBuf := make([]byte, 2^20*20) // 1MB
		// var fullLen int
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

			fmt.Printf("node sent %d bytes\n", n)
			writeHex(">", readBuf[:n])
			// printHex(">", readBuf[:n])

			if n == 0 {
				log.Fatalf("not expecting 0 bytes read")
			}

			fullBuf.Write(readBuf[:n])

			if n == 402 {
				continue
			}

			// log.Printf("===> WRITING FULL BUFFER ===>\n%x\n", fullBuf.Bytes())
			// printCbor(">", fullBuf.Bytes()[8:])

			_, err = client.Write(fullBuf.Bytes())
			if err != nil {
				log.Fatalf("%+v", errors.WithStack(err))
			}

			// fullLen = 0
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
			log.Fatalf("%+v", errors.WithStack(err3))
		}

		// printCbor("<", buf[:n])
		writeHex("<", buf[:n])
		// printHex("<", buf[:n])
		_, err = node.Write(buf[:n])
		if err != nil {
			log.Fatalf("%+v", errors.WithStack(err))
		}

		// time.Sleep(time.Second * 3)
	}
}

var requestIndex = new(atomic.Uint64)

func writeHex(direction string, buf []byte) {
	i := requestIndex.Add(1)
	d := "o"
	if direction == ">" {
		d = "i"
	}
	filename := fmt.Sprintf("%04d-%s-%d.dat", i, d, len(buf))
	fpath := path.Join(dir, filename)
	err := os.WriteFile(fpath, []byte(fmt.Sprintf("%x", buf)), os.ModePerm)
	fmt.Printf("wrote: %s\n", filename)
	if err != nil {
		log.Fatalf("%+v", errors.WithStack(err))
	}
}

func printHex(direction string, buf []byte) {
	var d string
	if direction == "<" {
		d = "<--"
	} else {
		d = "-->"
	}
	fmt.Printf("%s %x\n", d, buf)
}

func printCbor(direction string, buf []byte) {
	var d string
	if direction == "<" {
		d = "<--"
	} else {
		d = "-->"
	}
	text, _, err := cbor.DiagnoseFirst(buf)
	if err == nil {
		fmt.Printf("%s\n%s\n", d, text)
	}
}
