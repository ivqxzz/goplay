package main

func le32(b []byte, off int) uint32 {
	return uint32(b[off]) | uint32(b[off+1])<<8 | uint32(b[off+2])<<16 | uint32(b[off+3])<<24
}

func putLe32(b []byte, off int, v uint32) {
	b[off] = byte(v)
	b[off+1] = byte(v >> 8)
	b[off+2] = byte(v >> 16)
	b[off+3] = byte(v >> 24)
}

func keyByte(w uint32, i int) byte { return byte(w >> (8 * uint(i))) }

func swap_bytes(b []byte, i, j int) { b[i], b[j] = b[j], b[i] }

var sap_iv = []byte{
	0x2B, 0x84, 0xFB, 0x79, 0xDA, 0x75, 0xB9, 0x04, 0x6C, 0x24, 0x73, 0xF7, 0xD1, 0xC4, 0xAB, 0x0E,
	0x2B, 0x84, 0xFB, 0x79, 0x75, 0xB9, 0x04, 0x6C, 0x24, 0x73,
}

var sap_key_material = []byte{
	0xA1, 0x1A, 0x4A, 0x83,
	0xF2, 0x7A, 0x75, 0xEE,
	0xA2, 0x1A, 0x7D, 0xB8,
	0x8D, 0x77, 0x92, 0xAB,
}

var index_mangle = []byte{0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80, 0x1B, 0x36, 0x6C}

var initial_session_key = []byte{
	0xDC, 0xDC, 0xF3, 0xB9, 0x0B, 0x74, 0xDC, 0xFB, 0x86, 0x7F, 0xF7, 0x60, 0x16, 0x72, 0x90, 0x51,
}

var static_source_1 = []byte{
	0xFA, 0x9C, 0xAD, 0x4D, 0x4B, 0x68, 0x26, 0x8C, 0x7F, 0xF3, 0x88, 0x99, 0xDE, 0x92, 0x2E, 0x95,
	0x1E,
}

var static_source_2 = []byte{
	0xEC, 0x4E, 0x27, 0x5E, 0xFD, 0xF2, 0xE8, 0x30, 0x97, 0xAE, 0x70, 0xFB, 0xE0, 0x00, 0x3F, 0x1C,
	0x39, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

var default_sap = []byte{
	0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79,
	0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79,
	0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79,
	0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79, 0x79,
	0x79, 0x79, 0x79, 0x79, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x03, 0x02, 0x53,
	0x00, 0x01, 0xcc, 0x34, 0x2a, 0x5e, 0x5b, 0x1a, 0x67, 0x73, 0xc2, 0x0e, 0x21, 0xb8, 0x22, 0x4d,
	0xf8, 0x62, 0x48, 0x18, 0x64, 0xef, 0x81, 0x0a, 0xae, 0x2e, 0x37, 0x03, 0xc8, 0x81, 0x9c, 0x23,
	0x53, 0x9d, 0xe5, 0xf5, 0xd7, 0x49, 0xbc, 0x5b, 0x7a, 0x26, 0x6c, 0x49, 0x62, 0x83, 0xce, 0x7f,
	0x03, 0x93, 0x7a, 0xe1, 0xf6, 0x16, 0xde, 0x0c, 0x15, 0xff, 0x33, 0x8c, 0xca, 0xff, 0xb0, 0x9e,
	0xaa, 0xbb, 0xe4, 0x0f, 0x5d, 0x5f, 0x55, 0x8f, 0xb9, 0x7f, 0x17, 0x31, 0xf8, 0xf7, 0xda, 0x60,
	0xa0, 0xec, 0x65, 0x79, 0xc3, 0x3e, 0xa9, 0x83, 0x12, 0xc3, 0xb6, 0x71, 0x35, 0xa6, 0x69, 0x4f,
	0xf8, 0x23, 0x05, 0xd9, 0xba, 0x5c, 0x61, 0x5f, 0xa2, 0x54, 0xd2, 0xb1, 0x83, 0x45, 0x83, 0xce,
	0xe4, 0x2d, 0x44, 0x26, 0xc8, 0x35, 0xa7, 0xa5, 0xf6, 0xc8, 0x42, 0x1c, 0x0d, 0xa3, 0xf1, 0xc7,
	0x00, 0x50, 0xf2, 0xe5, 0x17, 0xf8, 0xd0, 0xfa, 0x77, 0x8d, 0xfb, 0x82, 0x8d, 0x40, 0xc7, 0x8e,
	0x94, 0x1e, 0x1e, 0x1e,
}

func xor_blocks(a, b, out []byte) {
	for i := 0; i < 16; i++ {
		out[i] = a[i] ^ b[i]
	}
}

func z_xor(in, out []byte, blocks int) {
	for j := 0; j < blocks; j++ {
		for i := 0; i < 16; i++ {
			out[j*16+i] = in[j*16+i] ^ z_key[i]
		}
	}
}

func x_xor(in, out []byte, blocks int) {
	for j := 0; j < blocks; j++ {
		for i := 0; i < 16; i++ {
			out[j*16+i] = in[j*16+i] ^ x_key[i]
		}
	}
}

func t_xor(in, out []byte) {
	for i := 0; i < 16; i++ {
		out[i] = in[i] ^ t_key[i]
	}
}

func table_index(i int) []byte         { return table_s1[((31*i)%0x28)<<8:] }
func message_table_index(i int) []byte { return table_s2[(97*i%144)<<8:] }
func permute_table_2(i int) []byte     { return table_s4[((71*i)%144)<<8:] }

func permute_block_1(block []byte) {
	block[0] = table_s3[int(block[0])]
	block[4] = table_s3[0x400+int(block[4])]
	block[8] = table_s3[0x800+int(block[8])]
	block[12] = table_s3[0xc00+int(block[12])]

	tmp := block[13]
	block[13] = table_s3[0x100+int(block[9])]
	block[9] = table_s3[0xd00+int(block[5])]
	block[5] = table_s3[0x900+int(block[1])]
	block[1] = table_s3[0x500+int(tmp)]

	tmp = block[2]
	block[2] = table_s3[0xa00+int(block[10])]
	block[10] = table_s3[0x200+int(tmp)]
	tmp = block[6]
	block[6] = table_s3[0xe00+int(block[14])]
	block[14] = table_s3[0x600+int(tmp)]

	tmp = block[3]
	block[3] = table_s3[0xf00+int(block[7])]
	block[7] = table_s3[0x300+int(block[11])]
	block[11] = table_s3[0x700+int(block[15])]
	block[15] = table_s3[0xb00+int(tmp)]
}

func permute_block_2(block []byte, round int) {
	block[0] = permute_table_2(round*16 + 0)[block[0]]
	block[4] = permute_table_2(round*16 + 4)[block[4]]
	block[8] = permute_table_2(round*16 + 8)[block[8]]
	block[12] = permute_table_2(round*16 + 12)[block[12]]

	tmp := block[13]
	block[13] = permute_table_2(round*16 + 13)[block[9]]
	block[9] = permute_table_2(round*16 + 9)[block[5]]
	block[5] = permute_table_2(round*16 + 5)[block[1]]
	block[1] = permute_table_2(round*16 + 1)[tmp]

	tmp = block[2]
	block[2] = permute_table_2(round*16 + 2)[block[10]]
	block[10] = permute_table_2(round*16 + 10)[tmp]
	tmp = block[6]
	block[6] = permute_table_2(round*16 + 6)[block[14]]
	block[14] = permute_table_2(round*16 + 14)[tmp]

	tmp = block[3]
	block[3] = permute_table_2(round*16 + 3)[block[7]]
	block[7] = permute_table_2(round*16 + 7)[block[11]]
	block[11] = permute_table_2(round*16 + 11)[block[15]]
	block[15] = permute_table_2(round*16 + 15)[tmp]
}

func generate_key_schedule(key_material []byte, key_schedule *[11][4]uint32) {
	var buffer [16]byte
	for i := 0; i < 11; i++ {
		key_schedule[i][0] = 0xdeadbeef
		key_schedule[i][1] = 0xdeadbeef
		key_schedule[i][2] = 0xdeadbeef
		key_schedule[i][3] = 0xdeadbeef
	}
	ti := 0
	t_xor(key_material, buffer[:])

	for round := 0; round < 11; round++ {

		key_schedule[round][0] = le32(buffer[:], 0)

		table1 := table_index(ti)
		table2 := table_index(ti + 1)
		table3 := table_index(ti + 2)
		table4 := table_index(ti + 3)
		ti += 4
		buffer[0] ^= table1[buffer[0x0d]] ^ index_mangle[round]
		buffer[1] ^= table2[buffer[0x0e]]
		buffer[2] ^= table3[buffer[0x0f]]
		buffer[3] ^= table4[buffer[0x0c]]

		key_schedule[round][1] = le32(buffer[:], 4)

		putLe32(buffer[:], 4, le32(buffer[:], 4)^le32(buffer[:], 0))

		key_schedule[round][2] = le32(buffer[:], 8)

		putLe32(buffer[:], 8, le32(buffer[:], 8)^le32(buffer[:], 4))

		key_schedule[round][3] = le32(buffer[:], 12)

		putLe32(buffer[:], 12, le32(buffer[:], 12)^le32(buffer[:], 8))
	}
}

func cycle(block []byte, key_schedule *[11][4]uint32) {
	putLe32(block, 0, le32(block, 0)^key_schedule[10][0])
	putLe32(block, 4, le32(block, 4)^key_schedule[10][1])
	putLe32(block, 8, le32(block, 8)^key_schedule[10][2])
	putLe32(block, 12, le32(block, 12)^key_schedule[10][3])

	permute_block_1(block)

	for round := 0; round < 9; round++ {
		k0 := key_schedule[9-round][0]
		ptr1 := table_s5[block[3]^keyByte(k0, 3)]
		ptr2 := table_s6[block[2]^keyByte(k0, 2)]
		ptr3 := table_s8[block[0]^keyByte(k0, 0)]
		ptr4 := table_s7[block[1]^keyByte(k0, 1)]
		ab := ptr1 ^ ptr2 ^ ptr3 ^ ptr4
		putLe32(block, 0, ab)

		k1 := key_schedule[9-round][1]
		ptr2 = table_s5[block[7]^keyByte(k1, 3)]
		ptr1 = table_s6[block[6]^keyByte(k1, 2)]
		ptr4 = table_s7[block[5]^keyByte(k1, 1)]
		ptr3 = table_s8[block[4]^keyByte(k1, 0)]
		ab = ptr1 ^ ptr2 ^ ptr3 ^ ptr4

		k2 := key_schedule[9-round][2]
		k3 := key_schedule[9-round][3]
		putLe32(block, 4, ab)

		putLe32(block, 8, table_s5[block[11]^keyByte(k2, 3)]^
			table_s6[block[10]^keyByte(k2, 2)]^
			table_s7[block[9]^keyByte(k2, 1)]^
			table_s8[block[8]^keyByte(k2, 0)])

		putLe32(block, 12, table_s5[block[15]^keyByte(k3, 3)]^
			table_s6[block[14]^keyByte(k3, 2)]^
			table_s7[block[13]^keyByte(k3, 1)]^
			table_s8[block[12]^keyByte(k3, 0)])

		permute_block_2(block, 8-round)
	}

	putLe32(block, 0, le32(block, 0)^key_schedule[0][0])
	putLe32(block, 4, le32(block, 4)^key_schedule[0][1])
	putLe32(block, 8, le32(block, 8)^key_schedule[0][2])
	putLe32(block, 12, le32(block, 12)^key_schedule[0][3])
}

func decryptMessage(messageIn, decryptedMessage []byte) {
	var buffer [16]byte
	mode := int(messageIn[12])

	var key_schedule [11][4]uint32
	generate_key_schedule(initial_session_key, &key_schedule)

	for i := 0; i < 8; i++ {
		for j := 0; j < 16; j++ {
			if mode == 3 {
				buffer[j] = messageIn[(0x80-0x10*i)+j]
			} else {
				buffer[j] = messageIn[(0x10*(i+1))+j]
			}
		}

		for j := 0; j < 9; j++ {
			base := 0x80 - 0x10*j
			mk := message_key[mode]

			buffer[0x0] = message_table_index(base + 0x0)[buffer[0x0]] ^ mk[base+0x0]
			buffer[0x4] = message_table_index(base + 0x4)[buffer[0x4]] ^ mk[base+0x4]
			buffer[0x8] = message_table_index(base + 0x8)[buffer[0x8]] ^ mk[base+0x8]
			buffer[0xc] = message_table_index(base + 0xc)[buffer[0xc]] ^ mk[base+0xc]

			tmp := buffer[0x0d]
			buffer[0xd] = message_table_index(base + 0xd)[buffer[0x9]] ^ mk[base+0xd]
			buffer[0x9] = message_table_index(base + 0x9)[buffer[0x5]] ^ mk[base+0x9]
			buffer[0x5] = message_table_index(base + 0x5)[buffer[0x1]] ^ mk[base+0x5]
			buffer[0x1] = message_table_index(base + 0x1)[tmp] ^ mk[base+0x1]

			tmp = buffer[0x02]
			buffer[0x2] = message_table_index(base + 0x2)[buffer[0xa]] ^ mk[base+0x2]
			buffer[0xa] = message_table_index(base + 0xa)[tmp] ^ mk[base+0xa]
			tmp = buffer[0x06]
			buffer[0x6] = message_table_index(base + 0x6)[buffer[0xe]] ^ mk[base+0x6]
			buffer[0xe] = message_table_index(base + 0xe)[tmp] ^ mk[base+0xe]

			tmp = buffer[0x3]
			buffer[0x3] = message_table_index(base + 0x3)[buffer[0x7]] ^ mk[base+0x3]
			buffer[0x7] = message_table_index(base + 0x7)[buffer[0xb]] ^ mk[base+0x7]
			buffer[0xb] = message_table_index(base + 0xb)[buffer[0xf]] ^ mk[base+0xb]
			buffer[0xf] = message_table_index(base + 0xf)[tmp] ^ mk[base+0xf]

			b0 := table_s9[0x000+int(buffer[0x0])] ^ table_s9[0x100+int(buffer[0x1])] ^
				table_s9[0x200+int(buffer[0x2])] ^ table_s9[0x300+int(buffer[0x3])]
			b1 := table_s9[0x000+int(buffer[0x4])] ^ table_s9[0x100+int(buffer[0x5])] ^
				table_s9[0x200+int(buffer[0x6])] ^ table_s9[0x300+int(buffer[0x7])]
			b2 := table_s9[0x000+int(buffer[0x8])] ^ table_s9[0x100+int(buffer[0x9])] ^
				table_s9[0x200+int(buffer[0xa])] ^ table_s9[0x300+int(buffer[0xb])]
			b3 := table_s9[0x000+int(buffer[0xc])] ^ table_s9[0x100+int(buffer[0xd])] ^
				table_s9[0x200+int(buffer[0xe])] ^ table_s9[0x300+int(buffer[0xf])]
			putLe32(buffer[:], 0, b0)
			putLe32(buffer[:], 4, b1)
			putLe32(buffer[:], 8, b2)
			putLe32(buffer[:], 12, b3)
		}

		buffer[0x0] = table_s10[(0x0<<8)+int(buffer[0x0])]
		buffer[0x4] = table_s10[(0x4<<8)+int(buffer[0x4])]
		buffer[0x8] = table_s10[(0x8<<8)+int(buffer[0x8])]
		buffer[0xc] = table_s10[(0xc<<8)+int(buffer[0xc])]

		tmp := buffer[0x0d]
		buffer[0xd] = table_s10[(0xd<<8)+int(buffer[0x9])]
		buffer[0x9] = table_s10[(0x9<<8)+int(buffer[0x5])]
		buffer[0x5] = table_s10[(0x5<<8)+int(buffer[0x1])]
		buffer[0x1] = table_s10[(0x1<<8)+int(tmp)]

		tmp = buffer[0x02]
		buffer[0x2] = table_s10[(0x2<<8)+int(buffer[0xa])]
		buffer[0xa] = table_s10[(0xa<<8)+int(tmp)]
		tmp = buffer[0x06]
		buffer[0x6] = table_s10[(0x6<<8)+int(buffer[0xe])]
		buffer[0xe] = table_s10[(0xe<<8)+int(tmp)]

		tmp = buffer[0x3]
		buffer[0x3] = table_s10[(0x3<<8)+int(buffer[0x7])]
		buffer[0x7] = table_s10[(0x7<<8)+int(buffer[0xb])]
		buffer[0xb] = table_s10[(0xb<<8)+int(buffer[0xf])]
		buffer[0xf] = table_s10[(0xf<<8)+int(tmp)]

		if mode == 2 || mode == 1 || mode == 0 {
			if i > 0 {
				xor_blocks(buffer[:], messageIn[0x10*i:], decryptedMessage[0x10*i:])
			} else {
				xor_blocks(buffer[:], message_iv[mode], decryptedMessage[0x10*i:])
			}
		} else {
			if i < 7 {
				xor_blocks(buffer[:], messageIn[0x70-0x10*i:], decryptedMessage[0x70-0x10*i:])
			} else {
				xor_blocks(buffer[:], message_iv[mode], decryptedMessage[0x70-0x10*i:])
			}
		}
	}
}

func generate_session_key(oldSap, messageIn, sessionKey []byte) {
	var decryptedMessage [128]byte
	var newSap [320]byte
	var md5 [16]byte

	decryptMessage(messageIn, decryptedMessage[:])

	copy(newSap[0x000:], static_source_1[:0x11])
	copy(newSap[0x011:], decryptedMessage[:0x80])
	copy(newSap[0x091:], oldSap[0x80:0x80+0x80])
	copy(newSap[0x111:], static_source_2[:0x2f])
	copy(sessionKey, initial_session_key[:16])

	for round := 0; round < 5; round++ {
		base := newSap[round*64 : round*64+64]
		modified_md5(base, sessionKey, md5[:])
		sap_hash(base, sessionKey)
		for i := 0; i < 4; i++ {
			sum := le32(sessionKey, i*4) + le32(md5[:], i*4)
			putLe32(sessionKey, i*4, sum)
		}
	}

	for i := 0; i < 16; i += 4 {
		swap_bytes(sessionKey, i, i+3)
		swap_bytes(sessionKey, i+1, i+2)
	}
	for i := 0; i < 16; i++ {
		sessionKey[i] ^= 121
	}
}
