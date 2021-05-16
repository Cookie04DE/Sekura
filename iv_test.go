package rubberhose_test

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"

	rubberhose "github.com/Cookie04DE/RubberHose"
	"github.com/stretchr/testify/require"
)

type ivTestCase struct {
	bytes          []byte
	number         int64
	expectedResult []byte
}

var ivTestCases = []ivTestCase{
	{bytes: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, number: 10, expectedResult: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10}},
	{bytes: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, number: 10, expectedResult: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11}},
	{bytes: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0}, number: 10, expectedResult: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 10}},
	{bytes: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255}, number: 1, expectedResult: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0}},
	{bytes: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255}, number: 2, expectedResult: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1}},
	{bytes: []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, number: 1, expectedResult: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
}

func TestIncrementIV(t *testing.T) {
	for i, testCase := range ivTestCases {
		rubberhose.IncrementIV(testCase.bytes, testCase.number)
		require.Equal(t, testCase.expectedResult, testCase.bytes, fmt.Sprintf("Test %d", i+1))
	}
}

func TestIncrementedIV(t *testing.T) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	iv := make([]byte, 16)
	_, err = rand.Read(iv)
	require.NoError(t, err)
	b, err := aes.NewCipher(key)
	require.NoError(t, err)
	ctr := cipher.NewCTR(b, iv)
	cipherText := make([]byte, 32)
	repeatedAs := strings.Repeat("a", 16)
	repeatedBs := strings.Repeat("b", 16)
	ctr.XORKeyStream(cipherText, []byte(repeatedAs+repeatedBs))
	ctr = cipher.NewCTR(b, iv)
	plainA := make([]byte, 16)
	ctr.XORKeyStream(plainA, cipherText[:16])
	require.Equal(t, repeatedAs, string(plainA))
	rubberhose.IncrementIV(iv, 1)
	ctr = cipher.NewCTR(b, iv)
	plainB := make([]byte, 16)
	ctr.XORKeyStream(plainB, cipherText[16:])
	require.Equal(t, repeatedBs, string(plainB))
}

func BenchmarkIncrementIV(b *testing.B) {
	iv := make([]byte, 16)
	_, err := rand.Read(iv)
	if err != nil {
		b.Fatal(err)
	}
	numberBytes := make([]byte, 8)
	_, err = rand.Read(iv)
	if err != nil {
		b.Fatal(err)
	}
	for n := 0; n < b.N; n++ {
		rubberhose.IncrementIV(iv, int64(binary.LittleEndian.Uint64(numberBytes)))
	}
}
