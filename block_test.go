package rubberhose_test

import (
	"crypto/rand"
	"os"
	"strings"
	"testing"

	rubberhose "github.com/Cookie04DE/RubberHose"
	"github.com/stretchr/testify/require"
)

func TestBlock(t *testing.T) {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)
	key := make([]byte, 32)
	_, err = rand.Read(key)
	require.NoError(t, err)
	d := rubberhose.NewDiskFromFile(f)
	block, err := rubberhose.NewBlock(d, key, 0, 0, rubberhose.MinBlockSize+32)
	require.NoError(t, err)
	err = block.Validate()
	require.Error(t, err)
	err = block.Write(-1)
	require.NoError(t, err)
	err = block.Validate()
	require.NoError(t, err)
	nextBlock, err := block.GetNextBlockID()
	require.NoError(t, err)
	require.Equal(t, int64(-1), nextBlock)
	_, err = block.WriteAt([]byte("a"), 0)
	require.NoError(t, err)
	buf := make([]byte, 1)
	_, err = block.ReadAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, "a", string(buf))

	repeatedAs := strings.Repeat("a", 16)
	repeatedBs := strings.Repeat("b", 16)
	_, err = block.WriteAt([]byte(repeatedAs+repeatedBs), 0)
	require.NoError(t, err)
	plainA := make([]byte, 16)
	_, err = block.ReadAt(plainA, 0)
	require.NoError(t, err)
	require.Equal(t, repeatedAs, string(plainA))
	plainB := make([]byte, 16)
	_, err = block.ReadAt(plainB, 16)
	require.NoError(t, err)
	require.Equal(t, repeatedBs, string(plainB))
	middleBs := make([]byte, 8)
	_, err = block.ReadAt(middleBs, 20)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("b", 8), string(middleBs))
}
