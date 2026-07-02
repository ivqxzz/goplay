package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"strconv"
)

type mirrorDecryptor struct {
	stream cipher.Stream
}

func newMirrorDecryptor(aesKey, ecdhSecret []byte, streamConnectionID uint64) (*mirrorDecryptor, error) {
	h := sha512.New()
	h.Write(aesKey[:16])
	h.Write(ecdhSecret[:32])
	eaeskey := h.Sum(nil)[:16]

	streamID := strconv.FormatUint(streamConnectionID, 10)

	hk := sha512.New()
	hk.Write([]byte("AirPlayStreamKey"))
	hk.Write([]byte(streamID))
	hk.Write(eaeskey)
	key := hk.Sum(nil)[:16]

	hi := sha512.New()
	hi.Write([]byte("AirPlayStreamIV"))
	hi.Write([]byte(streamID))
	hi.Write(eaeskey)
	iv := hi.Sum(nil)[:16]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return &mirrorDecryptor{stream: cipher.NewCTR(block, iv)}, nil
}

func (d *mirrorDecryptor) decryptFrame(buf []byte) {
	d.stream.XORKeyStream(buf, buf)
}

func reframeNALs(buf []byte) {
	i := 0
	for i+4 <= len(buf) {
		ncLen := binary.BigEndian.Uint32(buf[i : i+4])
		buf[i] = 0x00
		buf[i+1] = 0x00
		buf[i+2] = 0x00
		buf[i+3] = 0x01
		i += 4 + int(ncLen)
	}
}

func buildSPSPPS(payload []byte) []byte {
	spsSize := int(payload[6])<<8 | int(payload[7])
	sps := payload[8 : 8+spsSize]
	ppsSize := int(payload[spsSize+10])
	pps := payload[spsSize+11 : spsSize+11+ppsSize]

	out := make([]byte, 0, 8+spsSize+ppsSize)
	out = append(out, 0x00, 0x00, 0x00, 0x01)
	out = append(out, sps...)
	out = append(out, 0x00, 0x00, 0x00, 0x01)
	out = append(out, pps...)
	return out
}

func fairplayDecrypt(ekey []byte) ([]byte, error) {
	if len(ekey) < 72 {
		return nil, fmt.Errorf("fairplayDecrypt: ekey too short: %d bytes (need >=72)", len(ekey))
	}
	out := make([]byte, 16)
	playfair_decrypt(fpKeyMsg, ekey, out)
	return out, nil
}
