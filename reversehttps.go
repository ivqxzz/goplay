package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"howett.net/plist"
)

const fcupUserAgent = "AppleCoreMedia/1.0.0.11B554a (Apple TV; U; CPU OS 7_0_4 like Mac OS X; en_us"

type reverseChannel struct {
	mu     sync.Mutex
	conn   net.Conn
	reader *bufio.Reader

	sid   string
	alive bool
}

var reverseCh = &reverseChannel{}

func upgradeToReverse(conn net.Conn, reader *bufio.Reader, cseq int, sid string) {
	var b bytes.Buffer
	b.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	b.WriteString("Upgrade: PTTH/1.0\r\n")
	b.WriteString("Connection: Upgrade\r\n")
	fmt.Fprintf(&b, "CSeq: %d\r\n", cseq)
	b.WriteString("Server: AirTunes/220.68\r\n")
	b.WriteString("\r\n")

	if _, err := conn.Write(b.Bytes()); err != nil {
		log.Printf("reverse: failed to send 101 Switching Protocols: %v", err)
		conn.Close()
		return
	}
	log.Printf(">>> response 101 Switching Protocols | CSeq=%d | reverse channel PTTH/1.0", cseq)
	registerReverse(conn, reader, sid)
}

func registerReverse(conn net.Conn, reader *bufio.Reader, sid string) {
	reverseCh.mu.Lock()
	if reverseCh.alive && reverseCh.conn != nil && reverseCh.conn != conn {
		reverseCh.conn.Close()
	}
	reverseCh.conn = conn
	reverseCh.reader = reader
	reverseCh.sid = sid
	reverseCh.alive = true
	reverseCh.mu.Unlock()
	log.Printf("reverse: channel hijacked (sid=%q, %s)", sid, conn.RemoteAddr())
}

func closeReverse() {
	reverseCh.mu.Lock()
	defer reverseCh.mu.Unlock()
	if reverseCh.conn != nil {
		reverseCh.conn.Close()
	}
	reverseCh.conn = nil
	reverseCh.reader = nil
	reverseCh.alive = false
}

func reverseAlive() bool {
	reverseCh.mu.Lock()
	defer reverseCh.mu.Unlock()
	return reverseCh.alive && reverseCh.conn != nil
}

func reverseSID() string {
	reverseCh.mu.Lock()
	defer reverseCh.mu.Unlock()
	return reverseCh.sid
}

type fcupRequestInner struct {
	ClientInfo uint64            `plist:"FCUP_Response_ClientInfo"`
	ClientRef  uint64            `plist:"FCUP_Response_ClientRef"`
	RequestID  uint64            `plist:"FCUP_Response_RequestID"`
	URL        string            `plist:"FCUP_Response_URL"`
	SessionID  uint64            `plist:"sessionID"`
	Headers    map[string]string `plist:"FCUP_Response_Headers"`
}

type fcupRequestRoot struct {
	SessionID uint64           `plist:"sessionID"`
	Type      string           `plist:"type"`
	Request   fcupRequestInner `plist:"request"`
}

func buildFCUPRequestXML(mediaURL string, requestID int, clientSessionID string) ([]byte, error) {
	root := fcupRequestRoot{
		SessionID: 1,
		Type:      "unhandledURLRequest",
		Request: fcupRequestInner{
			ClientInfo: 1,
			ClientRef:  40030004,
			RequestID:  uint64(requestID),
			URL:        mediaURL,
			SessionID:  1,
			Headers: map[string]string{
				"X-Playback-Session-Id": clientSessionID,
				"User-Agent":            fcupUserAgent,
			},
		},
	}
	return plist.MarshalIndent(root, plist.XMLFormat, "  ")
}

func sendFCUPRequest(mediaURL string, requestID int, clientSessionID string) error {
	reverseCh.mu.Lock()
	defer reverseCh.mu.Unlock()

	if !reverseCh.alive || reverseCh.conn == nil {
		return fmt.Errorf("reverse: channel not established")
	}
	conn := reverseCh.conn
	sid := clientSessionID
	if sid == "" {
		sid = reverseCh.sid
	}

	body, err := buildFCUPRequestXML(mediaURL, requestID, sid)
	if err != nil {
		return fmt.Errorf("reverse: building FCUP plist: %w", err)
	}

	var req bytes.Buffer
	req.WriteString("POST /event HTTP/1.1\r\n")
	fmt.Fprintf(&req, "X-Apple-Session-ID: %s\r\n", sid)
	req.WriteString("Content-Type: text/x-apple-plist+xml\r\n")
	fmt.Fprintf(&req, "Content-Length: %d\r\n", len(body))
	req.WriteString("\r\n")
	req.Write(body)

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(req.Bytes()); err != nil {
		reverseCh.alive = false
		return fmt.Errorf("reverse: writing POST /event (req #%d): %w", requestID, err)
	}
	conn.SetWriteDeadline(time.Time{})

	log.Printf("reverse: >>> POST /event FCUP request #%d URL=%s (%d-byte plist)", requestID, mediaURL, len(body))
	return nil
}

func reverseGET(path string) ([]byte, string, error) {
	reverseCh.mu.Lock()
	defer reverseCh.mu.Unlock()

	if !reverseCh.alive || reverseCh.conn == nil || reverseCh.reader == nil {
		return nil, "", fmt.Errorf("reverse: channel not established")
	}
	conn := reverseCh.conn
	reader := reverseCh.reader

	if path == "" {
		path = "/"
	}

	var req bytes.Buffer
	fmt.Fprintf(&req, "GET %s HTTP/1.1\r\n", path)
	fmt.Fprintf(&req, "X-Apple-Session-ID: %s\r\n", reverseCh.sid)
	req.WriteString("User-Agent: AirPlay/220.68\r\n")
	req.WriteString("X-Apple-ProtocolVersion: 1\r\n")
	req.WriteString("Content-Length: 0\r\n")
	req.WriteString("\r\n")

	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(req.Bytes()); err != nil {
		reverseCh.alive = false
		return nil, "", fmt.Errorf("reverse: writing request %s: %w", path, err)
	}
	conn.SetWriteDeadline(time.Time{})
	log.Printf("reverse: >>> GET %s", path)

	conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		reverseCh.alive = false
		return nil, "", fmt.Errorf("reverse: reading response to %s: %w", path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		return nil, "", fmt.Errorf("reverse: reading body %s: %w", path, err)
	}

	ct := resp.Header.Get("Content-Type")
	log.Printf("reverse: <<< %s on GET %s | %d bytes | Content-Type=%s", resp.Status, path, len(data), ct)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, ct, fmt.Errorf("reverse: GET %s returned %s", path, resp.Status)
	}
	return data, ct, nil
}
