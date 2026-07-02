package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"io"
	"log"
	"net"
	"os/exec"
	"sync"
)

const (
	audioSampleRate   = 44100
	audioFrameSamples = 480
	audioFlushFrames  = 4
	audioDebugDump    = false
)

const audioInitSegmentHex = "0000001c6674797069736f6d0000020069736f6d69736f356d703432000002366d6f6f760000006c6d766864000000000000000000000000000003e80000000000010000010000000000000000000000000100000000000000000000000000000001000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000020000019a7472616b00000058746b686400000007000000000000000000000001000000000000000000000000000000000100000000010000000000000000000000000000000100000000000000000000000000004000000000000000000000000000013a6d646961000000206d6468640000000000000000000000000000ac440000000055c400000000002d68646c720000000000000000736f756e000000000000000000000000536f756e6448616e646c657200000000e56d696e6600000010736d686400000000000000000000002464696e660000001c6472656600000000000000010000000c75726c2000000001000000a97374626c0000005d7374736400000000000000010000004d6d703461000000000000000100000000000000000002001000000000ac440000000000296573647300000000031b0000000413401500000000000000000000000504f8e850000601020000001073747473000000000000000000000010737473630000000000000000000000147374737a000000000000000000000000000000107374636f0000000000000000000000286d7665780000002074726578000000000000000100000001000001e00000000000000000"

func be32(v uint32) []byte { b := make([]byte, 4); binary.BigEndian.PutUint32(b, v); return b }
func be64(v uint64) []byte { b := make([]byte, 8); binary.BigEndian.PutUint64(b, v); return b }

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func mp4box(typ string, payload []byte) []byte {
	out := be32(uint32(8 + len(payload)))
	out = append(out, typ...)
	out = append(out, payload...)
	return out
}

func mp4fbox(typ string, ver byte, flags uint32, payload []byte) []byte {
	head := be32((uint32(ver) << 24) | (flags & 0xFFFFFF))
	return mp4box(typ, append(head, payload...))
}

func deriveAudioKey() (key, iv []byte, ok bool) {
	if len(mirrorAESKey) < 16 || len(mirrorAESIV) < 16 || len(ecdhSecret) < 32 {
		return nil, nil, false
	}
	h := sha512.New()
	h.Write(mirrorAESKey[:16])
	h.Write(ecdhSecret[:32])
	key = h.Sum(nil)[:16]
	iv = make([]byte, 16)
	copy(iv, mirrorAESIV[:16])
	return key, iv, true
}

func decryptAudioFrame(block cipher.Block, baseIV, payload []byte) []byte {
	out := make([]byte, len(payload))
	copy(out, payload)
	n := (len(payload) / 16) * 16
	if n > 0 {
		iv := make([]byte, 16)
		copy(iv, baseIV)
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(out[:n], payload[:n])
	}
	return out
}

func audioFragment(seqNo uint32, baseDTS uint64, frames [][]byte) []byte {
	n := len(frames)
	buildTrun := func(dataOff uint32) []byte {
		p := cat(be32(uint32(n)), be32(dataOff))
		for _, f := range frames {
			p = append(p, be32(uint32(audioFrameSamples))...)
			p = append(p, be32(uint32(len(f)))...)
		}

		return mp4fbox("trun", 0, 0x000301, p)
	}
	tfhd := mp4fbox("tfhd", 0, 0x020000, be32(1))
	tfdt := mp4fbox("tfdt", 1, 0, be64(baseDTS))
	mfhd := mp4fbox("mfhd", 0, 0, be32(seqNo))

	traf0 := mp4box("traf", cat(tfhd, tfdt, buildTrun(0)))
	moof0 := mp4box("moof", cat(mfhd, traf0))
	dataOff := uint32(len(moof0) + 8)

	traf := mp4box("traf", cat(tfhd, tfdt, buildTrun(dataOff)))
	moof := mp4box("moof", cat(mfhd, traf))

	var mdatData []byte
	for _, f := range frames {
		mdatData = append(mdatData, f...)
	}
	mdat := mp4box("mdat", mdatData)
	return cat(moof, mdat)
}

func audioMpvArgs() []string {
	return []string{
		"--no-video",
		"--no-terminal",
		"--no-config",
		"--cache=no",
		"--idle=no",
		"--keep-open=no",
		"-",
	}
}

type audioPlayer struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	pc    net.PacketConn
}

var (
	audioMu      sync.Mutex
	audioCurrent *audioPlayer
)

func stopAudioLocked() {
	if audioCurrent == nil {
		return
	}
	old := audioCurrent
	audioCurrent = nil
	if old.stdin != nil {
		_ = old.stdin.Close()
	}
	if old.cmd != nil && old.cmd.Process != nil {
		_ = old.cmd.Process.Kill()
	}
	if old.pc != nil {
		_ = old.pc.Close()
	}
	log.Printf("audio: stopped previous mpv (new audio session started)")
}

func handleAudioPackets(pc net.PacketConn) {
	defer pc.Close()

	audioMu.Lock()
	stopAudioLocked()
	audioMu.Unlock()

	key, baseIV, ok := deriveAudioKey()
	if !ok {
		log.Printf("audio: no key material (aeskey/eiv/ecdh) — audio disabled")
		return
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Printf("audio: AES init error: %v", err)
		return
	}

	initSeg, err := hex.DecodeString(audioInitSegmentHex)
	if err != nil {
		log.Printf("audio: internal init-segment error: %v", err)
		return
	}

	mpvPath := findMirrorMpv()
	cmd := exec.Command(mpvPath, audioMpvArgs()...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("audio: failed to get mpv stdin: %v", err)
		return
	}
	if err := cmd.Start(); err != nil {
		log.Printf("audio: failed to start mpv(%s): %v", mpvPath, err)
		return
	}
	log.Printf("audio: started mpv(%s) for AAC-ELD (fMP4 -> stdin)", mpvPath)
	go cmd.Wait()

	audioMu.Lock()
	audioCurrent = &audioPlayer{cmd: cmd, stdin: stdin, pc: pc}
	audioMu.Unlock()

	defer func() {
		audioMu.Lock()
		if audioCurrent != nil && audioCurrent.cmd == cmd {
			audioCurrent = nil
		}
		audioMu.Unlock()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	w := bufio.NewWriterSize(stdin, 64*1024)
	if _, err := w.Write(initSeg); err != nil {
		log.Printf("audio: mpv rejected init-segment: %v", err)
		return
	}
	w.Flush()

	var (
		seen            = make(map[uint16]bool)
		order           = make([]uint16, 0, 2048)
		pending         = make([][]byte, 0, audioFlushFrames)
		seqNo    uint32 = 1
		dts      uint64
		buf      = make([]byte, 4096)
		totalPkt int
		totalByt int
	)

	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			log.Printf("audio: read stopped: %v", err)
			break
		}
		if n < 12 {
			continue
		}
		seq := binary.BigEndian.Uint16(buf[2:4])
		if seen[seq] {
			continue
		}
		seen[seq] = true
		order = append(order, seq)
		if len(order) > 1024 {
			delete(seen, order[0])
			order = order[1:]
		}

		frame := decryptAudioFrame(block, baseIV, buf[12:n])
		pending = append(pending, frame)
		totalPkt++
		totalByt += len(frame)

		if len(pending) >= audioFlushFrames {
			frag := audioFragment(seqNo, dts, pending)
			if _, err := w.Write(frag); err != nil {
				log.Printf("audio: mpv closed the stream: %v", err)
				break
			}
			if err := w.Flush(); err != nil {
				log.Printf("audio: error flushing to mpv: %v", err)
				break
			}
			seqNo++
			dts += uint64(audioFrameSamples * len(pending))
			pending = pending[:0]

			if totalPkt%500 < audioFlushFrames {
				log.Printf("audio: played ~%d frames, %d bytes ELD", totalPkt, totalByt)
			}
		}
	}

	w.Flush()
	stdin.Close()
}
