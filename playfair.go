package main

func playfair_decrypt(message, cipherText, keyOut []byte) {
	chunk1 := cipherText[16:]
	chunk2 := cipherText[56:]

	var sapKey [16]byte
	generate_session_key(default_sap, message, sapKey[:])

	var keySchedule [11][4]uint32
	generate_key_schedule(sapKey[:], &keySchedule)

	var blockIn [16]byte
	z_xor(chunk2, blockIn[:], 1)
	cycle(blockIn[:], &keySchedule)

	for i := 0; i < 16; i++ {
		keyOut[i] = blockIn[i] ^ chunk1[i]
	}

	x_xor(keyOut, keyOut, 1)
	z_xor(keyOut, keyOut, 1)
}
