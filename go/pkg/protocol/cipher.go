package protocol

import "encoding/binary"

const Ze uint32 = 1213550164

const (
	CompressFlag byte = 1
	BatchFlag    byte = 2
)

// XORCipher implements the 32-bit XORShift cipher used by the Huntera game engine.
func XORCipher(r []byte, seed uint32) {
	t := seed ^ Ze
	if t == 0 {
		t = Ze
	}

	alignedLen := len(r) & ^3
	a := 0
	for ; a < alignedLen; a += 4 {
		t ^= t << 13
		t ^= t >> 17
		t ^= t << 5
		val := binary.LittleEndian.Uint32(r[a : a+4])
		binary.LittleEndian.PutUint32(r[a:a+4], val^t)
	}

	if a < len(r) {
		t ^= t << 13
		t ^= t >> 17
		t ^= t << 5
		for ; a < len(r); a++ {
			r[a] ^= byte((t >> ((a & 3) * 8)) & 0xFF)
		}
	}
}
