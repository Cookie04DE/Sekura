package rubberhose

import (
	"encoding/binary"
)

func IncrementIV(iv []byte, n int64) {
	max := 8
	if len(iv) > max {
		max = len(iv)
	}
	nbytes := make([]byte, max)
	binary.BigEndian.PutUint64(nbytes[len(nbytes)-8:], uint64(n))
	carry := false
	for i := len(iv) - 1; i >= 0; i-- {
		old := iv[i]
		iv[i] += nbytes[i]
		if carry {
			iv[i]++
		}
		carry = old > iv[i]
	}
}
