package main

import "math"

var md5Shift = [64]int{
	7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
	5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
	4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
	6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
}

func md5F(b, c, d uint32) uint32 { return (b & c) | (^b & d) }
func md5G(b, c, d uint32) uint32 { return (b & d) | (c & ^d) }
func md5H(b, c, d uint32) uint32 { return b ^ c ^ d }
func md5I(b, c, d uint32) uint32 { return c ^ (b | ^d) }

func rol(input uint32, count int) uint32 {
	return (input << uint(count)) | (input >> uint(32-count))
}

func modified_md5(originalblockIn, keyIn, keyOut []byte) {
	var blockIn [64]byte
	copy(blockIn[:], originalblockIn[:64])

	swapWord := func(x, y int) {
		wx := le32(blockIn[:], x*4)
		wy := le32(blockIn[:], y*4)
		putLe32(blockIn[:], x*4, wy)
		putLe32(blockIn[:], y*4, wx)
	}

	A := le32(keyIn, 0)
	B := le32(keyIn, 4)
	C := le32(keyIn, 8)
	D := le32(keyIn, 12)

	for i := 0; i < 64; i++ {
		var j int
		switch {
		case i < 16:
			j = i
		case i < 32:
			j = (5*i + 1) % 16
		case i < 48:
			j = (3*i + 5) % 16
		default:
			j = (7 * i) % 16
		}

		input := uint32(blockIn[4*j])<<24 | uint32(blockIn[4*j+1])<<16 |
			uint32(blockIn[4*j+2])<<8 | uint32(blockIn[4*j+3])

		k := uint32(int64(float64(uint64(1)<<32) * math.Abs(math.Sin(float64(i+1)))))

		Z := A + input + k
		switch {
		case i < 16:
			Z = rol(Z+md5F(B, C, D), md5Shift[i])
		case i < 32:
			Z = rol(Z+md5G(B, C, D), md5Shift[i])
		case i < 48:
			Z = rol(Z+md5H(B, C, D), md5Shift[i])
		default:
			Z = rol(Z+md5I(B, C, D), md5Shift[i])
		}
		Z = Z + B

		tmp := D
		D = C
		C = B
		B = Z
		A = tmp

		if i == 31 {
			swapWord(int(A&15), int(B&15))
			swapWord(int(C&15), int(D&15))
			swapWord(int((A&(15<<4))>>4), int((B&(15<<4))>>4))
			swapWord(int((A&(15<<8))>>8), int((B&(15<<8))>>8))
			swapWord(int((A&(15<<12))>>12), int((B&(15<<12))>>12))
		}
	}

	putLe32(keyOut, 0, le32(keyIn, 0)+A)
	putLe32(keyOut, 4, le32(keyIn, 4)+B)
	putLe32(keyOut, 8, le32(keyIn, 8)+C)
	putLe32(keyOut, 12, le32(keyIn, 12)+D)
}
