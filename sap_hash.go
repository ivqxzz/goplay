package main

func rol8(input byte, count int) byte {
	return byte(((uint32(input) << uint(count)) & 0xff) | (uint32(input) >> uint(8-count)))
}

func rol8x(input byte, count int) uint32 {
	return (uint32(input) << uint(count)) | (uint32(input) >> uint(8-count))
}

func sap_hash(blockIn, keyOut []byte) {
	buffer0 := []byte{
		0x96, 0x5F, 0xC6, 0x53, 0xF8, 0x46, 0xCC, 0x18, 0xDF, 0xBE,
		0xB2, 0xF8, 0x38, 0xD7, 0xEC, 0x22, 0x03, 0xD1, 0x20, 0x8F,
	}
	var buffer1 [210]byte
	buffer2 := []byte{
		0x43, 0x54, 0x62, 0x7A, 0x18, 0xC3, 0xD6, 0xB3, 0x9A, 0x56,
		0xF6, 0x1C, 0x14, 0x3F, 0x0C, 0x1D, 0x3B, 0x36, 0x83, 0xB1,
		0x39, 0x51, 0x4A, 0xAA, 0x09, 0x3E, 0xFE, 0x44, 0xAF, 0xDE,
		0xC3, 0x20, 0x9D, 0x42, 0x3A,
	}
	var buffer3 [132]byte
	buffer4 := []byte{
		0xED, 0x25, 0xD1, 0xBB, 0xBC, 0x27, 0x9F, 0x02, 0xA2, 0xA9,
		0x11, 0x00, 0x0C, 0xB3, 0x52, 0xC0, 0xBD, 0xE3, 0x1B, 0x49,
		0xC7,
	}
	i0_index := []int{18, 22, 23, 0, 5, 19, 32, 31, 10, 21, 30}

	var w, x, y, z byte

	for i := 0; i < 210; i++ {
		inWord := le32(blockIn, ((i%64)>>2)*4)
		inByte := byte((inWord >> uint((3-(i%4))<<3)) & 0xff)
		buffer1[i] = inByte
	}

	for i := 0; i < 840; i++ {
		x = buffer1[uint32(i-155)%210]
		y = buffer1[uint32(i-57)%210]
		z = buffer1[uint32(i-13)%210]
		w = buffer1[uint32(i)%210]
		v := uint32(rol8(y, 5)) + (uint32(rol8(z, 3)) ^ uint32(w)) - uint32(rol8(x, 7))
		buffer1[i%210] = byte(v & 0xff)
	}

	garble(buffer0, buffer1[:], buffer2, buffer3[:], buffer4)

	for i := 0; i < 16; i++ {
		keyOut[i] = 0xE1
	}

	for i := 0; i < 11; i++ {
		if i == 3 {
			keyOut[i] = 0x3d
		} else {
			keyOut[i] = byte((uint32(keyOut[i]) + uint32(buffer3[i0_index[i]*4])) & 0xff)
		}
	}

	for i := 0; i < 20; i++ {
		keyOut[i%16] ^= buffer0[i]
	}

	for i := 0; i < 35; i++ {
		keyOut[i%16] ^= buffer2[i]
	}

	for i := 0; i < 210; i++ {
		keyOut[i%16] ^= buffer1[i]
	}

	for j := 0; j < 16; j++ {
		for i := 0; i < 16; i++ {
			x = keyOut[uint32(i-7)%16]
			y = keyOut[uint32(i)%16]
			z = keyOut[uint32(i-37)%16]
			w = keyOut[uint32(i-177)%16]
			keyOut[i] = rol8(x, 1) ^ y ^ rol8(z, 6) ^ rol8(w, 5)
		}
	}
}
