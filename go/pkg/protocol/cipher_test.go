package protocol

import (
	"bytes"
	"testing"
)

func TestXORCipherRoundtrip(t *testing.T) {
	lengths := []int{1, 2, 3, 4, 5, 7, 8, 9, 15, 16, 17, 32, 100, 1024}
	seeds := []uint32{0, 1, 42, 1213550164, 394820194, 4294967295}

	for _, l := range lengths {
		for _, seed := range seeds {
			original := make([]byte, l)
			for i := 0; i < l; i++ {
				original[i] = byte((i*37 + 13) % 256)
			}

			buf := make([]byte, l)
			copy(buf, original)

			// Encrypt
			XORCipher(buf, seed)

			// Should be different unless seed produces null (which XORShift avoids)
			if l > 4 && bytes.Equal(buf, original) {
				t.Fatalf("len %d seed %d: ciphertext equals plaintext", l, seed)
			}

			// Decrypt
			XORCipher(buf, seed)

			if !bytes.Equal(buf, original) {
				t.Fatalf("len %d seed %d: roundtrip failed, got %v want %v", l, seed, buf, original)
			}
		}
	}
}
