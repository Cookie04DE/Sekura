package rubberhose_test

import (
	"crypto/rand"
	"os"
	"testing"

	rubberhose "github.com/Cookie04DE/RubberHose"
	"github.com/stretchr/testify/require"
)

func TestPartition(t *testing.T) {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)
	key := make([]byte, 16)
	_, err = rand.Read(key)
	require.NoError(t, err)
	b1, err := rubberhose.NewBlock(f, key, 0, 0, rubberhose.MinBlockSize+1)
	require.NoError(t, err)
	err = b1.Write(1)
	require.NoError(t, err)
	b2, err := rubberhose.NewBlock(f, key, 0, 1, rubberhose.MinBlockSize+1)
	require.NoError(t, err)
	err = b2.Write(-1)
	require.NoError(t, err)
	partition := rubberhose.NewPartition(1, []*rubberhose.Block{b1, b2})
	_, err = partition.WriteAt([]byte("ab"), 0)
	require.NoError(t, err)
	buf := make([]byte, 1)
	_, err = partition.ReadAt(buf, 0)
	require.NoError(t, err)
	require.Equal(t, "a", string(buf))
	_, err = partition.ReadAt(buf, 1)
	require.NoError(t, err)
	require.Equal(t, "b", string(buf))
}
