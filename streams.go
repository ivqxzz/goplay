package main

import (
	"encoding/binary"
	"io"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"howett.net/plist"
)

type setupStreamReq struct {
	Type               int    `plist:"type"`
	StreamConnectionID uint64 `plist:"streamConnectionID"`
}

type setupReq struct {
	EIV        []byte           `plist:"eiv"`
	EKey       []byte           `plist:"ekey"`
	TimingPort int              `plist:"timingPort"`
	Streams    []setupStreamReq `plist:"streams"`
}

func handleSetup(conn net.Conn, body []byte) ([]byte, error) {
	var req setupReq
	if _, err := plist.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	clientHost, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
	res := map[string]interface{}{}

	if len(req.EKey) > 0 && len(req.EIV) > 0 {
		log.Printf("SETUP #1: ekey=%d bytes, eiv=%d bytes, timingPort(client)=%d",
			len(req.EKey), len(req.EIV), req.TimingPort)
		key, err := fairplayDecrypt(req.EKey)
		if err != nil {
			log.Printf("SETUP #1: fairplayDecrypt: %v", err)
		}
		mirrorAESKey = key
		mirrorAESIV = append([]byte(nil), req.EIV...)
		timingLPort := startTiming(clientHost, req.TimingPort)
		eventPort := ensureEventListener()
		res["eventPort"] = eventPort
		res["timingPort"] = timingLPort
		log.Printf("SETUP #1 response: eventPort=%d, timingPort=%d", eventPort, timingLPort)
	}

	if len(req.Streams) > 0 {
		streams := []map[string]interface{}{}
		for _, s := range req.Streams {
			switch s.Type {
			case 110:
				dec, err := newMirrorDecryptor(mirrorAESKey, ecdhSecret, s.StreamConnectionID)
				if err != nil {
					log.Printf("mirror: decryptor not created (%v) — writing as-is", err)
				}
				dport := startMirror(s.StreamConnectionID, dec)
				streams = append(streams, map[string]interface{}{
					"dataPort": dport,
					"type":     110,
				})
				log.Printf("SETUP #2 video stream (type=110): dataPort=%d", dport)
			case 96:
				dport, cport := startAudio()
				streams = append(streams, map[string]interface{}{
					"dataPort":    dport,
					"controlPort": cport,
					"type":        96,
				})
				log.Printf("SETUP #2 audio stream (type=96): dataPort=%d, controlPort=%d", dport, cport)
			default:
				log.Printf("SETUP: unknown stream type %d (skipping)", s.Type)
			}
		}
		res["streams"] = streams
	}

	return plist.Marshal(res, plist.BinaryFormat)
}

var (
	mirrorAESKey []byte
	mirrorAESIV  []byte
)

var (
	eventOnce sync.Once
	eventPort int
)

func ensureEventListener() int {
	eventOnce.Do(func() {
		ln, err := net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			log.Printf("event: failed to listen: %v", err)
			return
		}
		eventPort = ln.Addr().(*net.TCPAddr).Port
		log.Printf("event: listening on port %d", eventPort)
		go func() {
			for {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				go func(c net.Conn) { defer c.Close(); io.Copy(io.Discard, c) }(c)
			}
		}()
	})
	return eventPort
}

const secondsFrom1900To1970 = 2208988800

func ntpNow() uint64 {
	now := time.Now()
	sec := uint64(now.Unix()) + secondsFrom1900To1970
	frac := (uint64(now.Nanosecond()) << 32) / 1000000000
	return (sec << 32) | (frac & 0xffffffff)
}

func startTiming(clientHost string, clientPort int) int {
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		log.Printf("timing: failed to open UDP: %v", err)
		return 0
	}
	localPort := pc.LocalAddr().(*net.UDPAddr).Port
	log.Printf("timing: UDP port %d (polling client %s:%d)", localPort, clientHost, clientPort)
	if clientPort > 0 {
		remote := &net.UDPAddr{IP: net.ParseIP(clientHost), Port: clientPort}
		go ntpLoop(pc, remote)
	}
	return localPort
}

func ntpLoop(pc *net.UDPConn, remote *net.UDPAddr) {
	defer pc.Close()
	buf := make([]byte, 128)
	for {
		req := make([]byte, 32)
		req[0], req[1], req[2], req[3] = 0x80, 0xd2, 0x00, 0x07
		binary.BigEndian.PutUint64(req[24:32], ntpNow())
		if _, err := pc.WriteToUDP(req, remote); err != nil {
			log.Printf("timing: send error: %v", err)
		}
		pc.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := pc.ReadFromUDP(buf); err == nil {

		}
		time.Sleep(3 * time.Second)
	}
}

func startMirror(streamConnID uint64, dec *mirrorDecryptor) int {
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Printf("mirror: failed to listen: %v", err)
		return 0
	}
	port := ln.Addr().(*net.TCPAddr).Port
	log.Printf("mirror: listening for video stream on port %d (streamConnectionID=%d)", port, streamConnID)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			log.Printf("mirror: [+] video stream connected from %s", c.RemoteAddr())
			go readMirror(c, dec)
		}
	}()
	return port
}

func containsIDR(annexb []byte) bool {
	for i := 0; i+4 < len(annexb); i++ {
		if annexb[i] == 0 && annexb[i+1] == 0 && annexb[i+2] == 0 && annexb[i+3] == 1 {
			if annexb[i+4]&0x1f == 5 {
				return true
			}
			i += 3
		}
	}
	return false
}

func readMirror(c net.Conn, dec *mirrorDecryptor) {
	defer c.Close()

	player, perr := newVideoPlayer()
	if perr != nil {
		log.Printf("mirror: video window not started: %v", perr)
	}
	if player != nil {
		defer player.close()
	}

	var paramSet []byte
	needKeyframe := true
	dropped := 0

	header := make([]byte, 128)
	frames := 0
	for {
		if _, err := io.ReadFull(c, header); err != nil {
			log.Printf("mirror: [-] stream closed (%v), total video frames: %d", err, frames)
			return
		}
		payloadSize := binary.LittleEndian.Uint32(header[0:4])
		payloadType := header[4]
		if payloadSize == 0 {
			continue
		}
		payload := make([]byte, payloadSize)
		if _, err := io.ReadFull(c, payload); err != nil {
			log.Printf("mirror: error reading payload (%v)", err)
			return
		}

		switch payloadType {
		case 0:
			frames++
			if dec != nil {
				dec.decryptFrame(payload)
			}
			reframeNALs(payload)

			isIDR := containsIDR(payload)

			if needKeyframe {
				if !isIDR {
					dropped++
					continue
				}
				needKeyframe = false
				if dropped > 0 {
					log.Printf("mirror: keyframe received, playback resumed (dropped %d frames)", dropped)
					dropped = 0
				}
			}

			if player != nil && isIDR && paramSet != nil {
				if err := player.write(paramSet); err != nil {
					log.Printf("mirror: player exited (%v)", err)
					player = nil
				}
			}
			if player != nil {
				if err := player.write(payload); err != nil {
					log.Printf("mirror: player exited (%v)", err)
					player = nil
				}
			}
			if frames%30 == 1 {
				log.Printf("mirror: video frame #%d, %d bytes", frames, payloadSize)
			}

		case 1:
			ws := math.Float32frombits(binary.LittleEndian.Uint32(header[40:44]))
			hs := math.Float32frombits(binary.LittleEndian.Uint32(header[44:48]))
			w := math.Float32frombits(binary.LittleEndian.Uint32(header[56:60]))
			h := math.Float32frombits(binary.LittleEndian.Uint32(header[60:64]))
			log.Printf("mirror: SPS/PPS — source %.0fx%.0f, screen %.0fx%.0f", ws, hs, w, h)
			if annexb := buildSPSPPS(payload); annexb != nil {
				paramSet = annexb
				needKeyframe = true
				if player != nil {
					if err := player.write(annexb); err != nil {
						log.Printf("mirror: player exited (%v)", err)
						player = nil
					}
				}
			}

		default:

		}
	}
}

func startAudio() (int, int) {
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		log.Printf("audio: failed to open UDP (data): %v", err)
		return 0, 0
	}
	dataPort := pc.LocalAddr().(*net.UDPAddr).Port

	cc, cerr := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if cerr != nil {
		log.Printf("audio: failed to open UDP (control): %v", cerr)
		log.Printf("audio: listening for audio stream (UDP) data=%d (no control)", dataPort)
		go handleAudioPackets(pc)
		return dataPort, 0
	}
	ctrlPort := cc.LocalAddr().(*net.UDPAddr).Port

	log.Printf("audio: listening for audio stream (UDP) data=%d, control=%d", dataPort, ctrlPort)
	go handleAudioPackets(pc)
	go drainAudioControl(cc)
	return dataPort, ctrlPort
}

func drainAudioControl(cc *net.UDPConn) {
	defer cc.Close()
	buf := make([]byte, 2048)
	packets := 0
	for {
		n, addr, err := cc.ReadFromUDP(buf)
		if n > 0 {
			if packets < 8 {
				m := n
				if m > 32 {
					m = 32
				}
				log.Printf("audio-control: packet #%d from %s: %d bytes, first %d: % x", packets, addr, n, m, buf[:m])
			}
			packets++
		}
		if err != nil {
			log.Printf("audio-control: [-] closed (%v), packets: %d", err, packets)
			return
		}
	}
}

func dumpAudio(pc *net.UDPConn) {
	defer pc.Close()

	log.Printf("audio-key: aeskey(fairplay) % x", mirrorAESKey)
	log.Printf("audio-key: eiv % x", mirrorAESIV)
	log.Printf("audio-key: ecdh % x", ecdhSecret)

	dumpPath := filepath.Join(mirrorExeDir(), "audio_packets.bin")
	f, ferr := os.Create(dumpPath)
	if ferr != nil {
		log.Printf("audio: failed to create dump %s: %v", dumpPath, ferr)
	} else {
		defer f.Close()
		log.Printf("audio: writing packets (uint32-LE length prefix + packet) to %s", dumpPath)
	}

	buf := make([]byte, 4096)
	lenbuf := make([]byte, 4)
	total := 0
	packets := 0
	nextMark := 65536
	for {
		n, addr, err := pc.ReadFromUDP(buf)
		if n > 0 {
			if packets == 0 {
				log.Printf("audio: [+] first UDP packet from %s", addr)
			}
			if f != nil {
				binary.LittleEndian.PutUint32(lenbuf, uint32(n))
				f.Write(lenbuf)
				f.Write(buf[:n])
			}
			if packets < 16 {
				m := n
				if m > 48 {
					m = 48
				}
				log.Printf("audio: packet #%d: %d bytes, first %d: % x", packets, n, m, buf[:m])
			}
			packets++
			total += n
			if total >= nextMark {
				log.Printf("audio: accumulated %d bytes over %d packets", total, packets)
				nextMark += 65536
			}
		}
		if err != nil {
			log.Printf("audio: [-] stream closed (%v), total %d bytes over %d packets", err, total, packets)
			return
		}
	}
}
