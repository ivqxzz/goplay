package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"unicode"

	"howett.net/plist"
)

func serveRTSP(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	hijacked := false
	defer func() {
		if !hijacked {
			conn.Close()
		}
	}()
	log.Printf("[+] connection from %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)

	for {
		requestLine, err := tp.ReadLine()
		if err != nil {
			log.Printf("[-] %s closed (%v)", conn.RemoteAddr(), err)
			return
		}
		if requestLine == "" {
			continue
		}

		parts := strings.SplitN(requestLine, " ", 3)
		if len(parts) < 3 {
			log.Printf("malformed request line: %q", requestLine)
			return
		}
		method, uri, version := parts[0], parts[1], parts[2]

		headers, err := tp.ReadMIMEHeader()
		if err != nil {
			log.Printf("[-] %s error reading headers (%v)", conn.RemoteAddr(), err)
			return
		}

		cseq, _ := strconv.Atoi(headers.Get("CSeq"))
		contentType := headers.Get("Content-Type")
		sid := headers.Get("X-Apple-Session-ID")

		var body []byte
		if cl := headers.Get("Content-Length"); cl != "" {
			n, _ := strconv.Atoi(cl)
			if n > 0 {
				body = make([]byte, n)
				if _, err := io.ReadFull(reader, body); err != nil {
					log.Printf("[-] %s error reading body (%v)", conn.RemoteAddr(), err)
					return
				}
			}
		}

		log.Printf("=== %s %s %s | CSeq=%d | body %d bytes | Content-Type=%s | sid=%s",
			method, uri, version, cseq, len(body), contentType, sid)

		uriPath := uri
		if i := strings.IndexByte(uriPath, '?'); i >= 0 {
			uriPath = uriPath[:i]
		}
		if method == "POST" && strings.HasPrefix(uriPath, "/reverse") {
			upgradeToReverse(conn, reader, cseq, sid)
			hijacked = true
			return
		}

		respond(conn, method, uri, version, cseq, body)
	}
}

func dumpBody(tag string, body []byte) {
	if len(body) == 0 {
		return
	}
	const max = 800
	b := body
	truncated := false
	if len(b) > max {
		b = b[:max]
		truncated = true
	}
	var sb strings.Builder
	for _, c := range b {
		switch {
		case c == '\n' || c == '\r' || c == '\t':
			sb.WriteByte(' ')
		case c < 128 && unicode.IsPrint(rune(c)):
			sb.WriteByte(c)
		default:
			sb.WriteByte('.')
		}
	}
	tail := ""
	if truncated {
		tail = fmt.Sprintf(" …(+%d bytes)", len(body)-max)
	}
	log.Printf("    └─ %s body(%d): %s%s", tag, len(body), sb.String(), tail)
}

func respond(conn net.Conn, method, uri, version string, cseq int, body []byte) {

	proto := "RTSP/1.0"
	if strings.HasPrefix(version, "HTTP") {
		proto = "HTTP/1.1"
	}

	switch method {
	case "SETUP":
		out, err := handleSetup(conn, body)
		if err != nil {
			log.Printf("SETUP error: %v", err)
			writeResp(conn, proto, "400 Bad Request", cseq, "", nil)
			return
		}
		writeResp(conn, proto, "200 OK", cseq, "application/x-apple-binary-plist", out)
		return
	case "RECORD":
		writeResp(conn, proto, "200 OK", cseq, "", nil)
		return
	case "SET_PARAMETER":
		writeResp(conn, proto, "200 OK", cseq, "", nil)
		return
	case "GET_PARAMETER":
		resp := []byte("volume: 0.000000\r\n")
		writeResp(conn, proto, "200 OK", cseq, "text/parameters", resp)
		return
	case "FLUSH":
		writeResp(conn, proto, "200 OK", cseq, "", nil)
		return
	case "TEARDOWN":
		writeResp(conn, proto, "200 OK", cseq, "", nil)
		return
	case "OPTIONS":
		writeRespEx(conn, proto, "200 OK", cseq, "", nil, map[string]string{
			"Public": "ANNOUNCE, SETUP, RECORD, PAUSE, FLUSH, TEARDOWN, OPTIONS, GET_PARAMETER, SET_PARAMETER, POST, GET",
		})
		return
	}

	switch {
	case strings.HasPrefix(uri, "/server-info"):

		out, err := serverInfoPlist()
		if err != nil {
			log.Printf("server-info error: %v", err)
			writeResp(conn, proto, "500 Internal Server Error", cseq, "", nil)
			return
		}
		log.Printf("server-info: sending receiver capabilities (%d bytes)", len(out))
		writeResp(conn, proto, "200 OK", cseq, "text/x-apple-plist+xml", out)
		return
	case strings.HasPrefix(uri, "/fp-setup2"):

		dumpBody("POST /fp-setup2", body)
		log.Printf("fp-setup2: DRM stage not implemented, replying 200 (trying not to drop the session)")
		writeResp(conn, proto, "200 OK", cseq, "application/octet-stream", nil)
		return
	case strings.HasPrefix(uri, "/info"):
		out, err := infoPlist()
		if err != nil {
			log.Printf("info error: %v", err)
			writeResp(conn, proto, "500 Internal Server Error", cseq, "", nil)
			return
		}
		writeResp(conn, proto, "200 OK", cseq, "application/x-apple-binary-plist", out)
		return
	case strings.HasPrefix(uri, "/pair-setup"):
		writeResp(conn, proto, "200 OK", cseq, "application/octet-stream", pairSetup(body))
		return
	case strings.HasPrefix(uri, "/pair-verify"):
		out, err := pairVerify(body)
		if err != nil {
			log.Printf("pair-verify error: %v", err)
			writeResp(conn, proto, "400 Bad Request", cseq, "", nil)
			return
		}
		writeResp(conn, proto, "200 OK", cseq, "application/octet-stream", out)
		return
	case strings.HasPrefix(uri, "/fp-setup"):
		out, err := fairplaySetup(body)
		if err != nil {
			log.Printf("fp-setup error: %v", err)
			writeResp(conn, proto, "400 Bad Request", cseq, "", nil)
			return
		}
		writeResp(conn, proto, "200 OK", cseq, "application/octet-stream", out)
		return
	default:

		dumpBody(method+" "+uri, body)
		if out, ct, handled := handleAirplayVideo(method, uri, body); handled {
			writeResp(conn, proto, "200 OK", cseq, ct, out)
			return
		}
		log.Printf("unhandled request: %s %s", method, uri)
		writeResp(conn, proto, "200 OK", cseq, "", nil)
		return
	}
}

func serverInfoPlist() ([]byte, error) {
	info := map[string]interface{}{
		"deviceid":  deviceInfo["deviceID"],
		"features":  deviceInfo["features"],
		"model":     deviceInfo["model"],
		"srcvers":   deviceInfo["sourceVersion"],
		"protovers": "1.0",
		"vv":        deviceInfo["vv"],
	}
	return plist.Marshal(info, plist.XMLFormat)
}

func writeResp(conn net.Conn, proto, status string, cseq int, contentType string, body []byte) {
	writeRespEx(conn, proto, status, cseq, contentType, body, nil)
}

func writeRespEx(conn net.Conn, proto, status string, cseq int, contentType string, body []byte, extra map[string]string) {
	if proto == "" {
		proto = "RTSP/1.0"
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s %s\r\n", proto, status)
	fmt.Fprintf(&b, "CSeq: %d\r\n", cseq)
	fmt.Fprintf(&b, "Server: AirTunes/220.68\r\n")
	for k, v := range extra {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	if contentType != "" {
		fmt.Fprintf(&b, "Content-Type: %s\r\n", contentType)
	}
	fmt.Fprintf(&b, "Content-Length: %d\r\n", len(body))
	b.WriteString("\r\n")
	if len(body) > 0 {
		b.Write(body)
	}

	if _, err := conn.Write(b.Bytes()); err != nil {
		log.Printf("error writing response: %v", err)
		return
	}
	log.Printf(">>> response %s | CSeq=%d | body %d bytes", status, cseq, len(body))
}
