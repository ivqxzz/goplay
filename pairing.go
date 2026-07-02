package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"fmt"
	"log"

	"golang.org/x/crypto/curve25519"
)

var (
	accessoryPub  ed25519.PublicKey
	accessoryPriv ed25519.PrivateKey
)

var ecdhSecret []byte

func initPairing() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("ed25519 keygen: %v", err)
	}
	accessoryPub = pub
	accessoryPriv = priv
	log.Printf("pairing: device key ready, pub=%x", pub)
}

func pairSetup(_ []byte) []byte {
	return accessoryPub
}

func pairVerify(body []byte) ([]byte, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("body too short: %d", len(body))
	}

	if body[0] != 0 {
		if len(body) < 68 {
			return nil, fmt.Errorf("phase1: expected 68 bytes, got %d", len(body))
		}
		clientCurve := body[4:36]

		var ourPriv [32]byte
		if _, err := rand.Read(ourPriv[:]); err != nil {
			return nil, err
		}
		ourPub, err := curve25519.X25519(ourPriv[:], curve25519.Basepoint)
		if err != nil {
			return nil, err
		}

		shared, err := curve25519.X25519(ourPriv[:], clientCurve)
		if err != nil {
			return nil, err
		}

		ecdhSecret = append([]byte(nil), shared...)

		aesKey := sha512Half("Pair-Verify-AES-Key", shared)
		aesIV := sha512Half("Pair-Verify-AES-IV", shared)

		signMsg := make([]byte, 0, 64)
		signMsg = append(signMsg, ourPub...)
		signMsg = append(signMsg, clientCurve...)
		sig := ed25519.Sign(accessoryPriv, signMsg)

		block, err := aes.NewCipher(aesKey)
		if err != nil {
			return nil, err
		}
		ctr := cipher.NewCTR(block, aesIV)
		encSig := make([]byte, len(sig))
		ctr.XORKeyStream(encSig, sig)

		resp := make([]byte, 0, 96)
		resp = append(resp, ourPub...)
		resp = append(resp, encSig...)
		return resp, nil
	}

	return []byte{}, nil
}

func sha512Half(prefix string, data []byte) []byte {
	h := sha512.New()
	h.Write([]byte(prefix))
	h.Write(data)
	sum := h.Sum(nil)
	return sum[:16]
}
