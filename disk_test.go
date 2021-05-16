package rubberhose_test

import (
	"os"
	"testing"

	rubberhose "github.com/Cookie04DE/RubberHose"
	"github.com/stretchr/testify/require"
)

func TestDisk(t *testing.T) {
	f, err := os.CreateTemp("", "")
	require.NoError(t, err)
	d := rubberhose.NewDisk(f)
	err = d.Write(rubberhose.MinBlockSize+10, 10)
	require.NoError(t, err)
	testPass := "test"
	_, err = d.GetPartition(testPass)
	require.Error(t, err)
	p, err := d.WritePartition(testPass, 4)
	require.NoError(t, err)
	testBytes := []byte("Test write")
	_, err = p.WriteAt(testBytes, 0)
	require.NoError(t, err)
	p, err = d.GetPartition(testPass)
	require.NoError(t, err)
	readBytes := make([]byte, len(testBytes))
	_, err = p.ReadAt(readBytes, 0)
	require.NoError(t, err)
	require.Equal(t, string(testBytes), string(readBytes))
}
