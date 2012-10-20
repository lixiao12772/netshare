package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"text/template"
)

const (
	DEFAULT_HOST      = "127.0.0.1"
	DEFAULT_PORT      = "8080"
	DEFAULT_MIME_TYPE = "text/plain"
	HEAD_TMPL         = "HTTP/1.0 200 OK\nCache-Control: max-age=31536000\nExpires: Thu, 31 Dec 2037 23:55:55 GMT\nContent-Type: {{.Mime}}\nContent-Length: {{.Length}}\n\n"
)

var (
	src     *os.File
	size    int64
	headers string
)

func main() {
	log.Println("Start")

	var oerr error

	host, port, mimetype, filename := parseArgs()

	src, oerr = os.Open(filename)
	if oerr != nil {
		log.Fatal("Error opening payload. ", oerr)
	}

	fileinfo, serr := src.Stat()
	if serr != nil {
		log.Fatal("Error Stat on payload")
	}
	size = fileinfo.Size()
	log.Println("Payload size:", size)

	tmpl, terr := template.New("headers").Parse(HEAD_TMPL)
	if terr != nil {
		log.Fatal("Error parsing HEAD_TMPL", terr)
	}

	tmplData := struct {
		Mime   string
		Length int64
	}{mimetype, size}

	headBuf := &bytes.Buffer{}
	terr = tmpl.Execute(headBuf, tmplData)
	if terr != nil {
		log.Fatal("Error executing header template", terr)
	}

	headers = headBuf.String()

	// Warm up OS page cache
	ioutil.ReadAll(src)
	src.Seek(0, os.SEEK_SET)

	addr := host + ":" + port
	sock, lerr := net.Listen("tcp", addr)
	if lerr != nil {
		log.Fatal("Error listening on ", addr, ". ", lerr)
	}

	for {
		conn, aerr := sock.Accept()

		if aerr != nil {
			log.Fatal("Error Accept. ", aerr)
		}

		go handle(conn)
	}
}

func handle(conn net.Conn) {

	var pos int64
	var read, wrote int
	var rerr, werr error
	buf := make([]byte, 32*1024)

	wrote, werr = conn.Write([]byte(headers))
	if werr != nil {
		log.Fatal("Error writing headers", werr)
	}

	for pos < size {

		read, rerr = src.ReadAt(buf, pos)
		if rerr != nil && rerr != io.EOF {
			log.Println("Read error:", rerr)
			break
		}
		//log.Println("Read bytes:", read)

		pos += int64(read)

		wrote, werr = conn.Write(buf[:read])
		if werr != nil {
			log.Println("Write error:", werr)
			break
		}
		//log.Println("Wrote bytes:", wrote)

		if wrote != read {
			log.Println("Read / write mismatch. Read ", read, ", wrote ", wrote)
		}
	}

	//log.Println("CloseWrite")
	werr = conn.(*net.TCPConn).CloseWrite()
	if werr != nil {
		log.Println("Error on CloseWrite", werr)
	}

	// Consume the input
	for {
		read, rerr = conn.Read(buf)
		if read == 0 {
			break
		}
	}

	werr = conn.Close()
	if werr != nil {
		log.Println("Error on Close", werr)
	}

}

func parseArgs() (host, port, mimetype, filename string) {

	flag.Usage = Usage

	hostf := flag.String("h", DEFAULT_HOST, "Host or IP to listen on")
	portf := flag.String("p", DEFAULT_PORT, "Port to listen on")
	mimetypef := flag.String("m", DEFAULT_MIME_TYPE, "Mime type of file")

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(1)
	}

	flag.Parse()

	return *hostf, *portf, *mimetypef, flag.Arg(0)
}

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "  %s [options] [filename]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Options:\n")

	flag.PrintDefaults()
}
