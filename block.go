package rubberhose

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

var blockStartingMagic = []byte{144, 53, 207, 44, 57, 127, 48, 142}

const ( //in bytes
	ivOffset = 0
	ivSize   = 16

	blockMagicOffset = ivOffset + ivSize
	blockMagicSize   = 8 //same length as blockStartingMagic

	nextBlockIDOffset = blockMagicOffset + blockMagicSize //This is the offset where the blockID of the next block is stored
	blockIDSize       = 8                                 //saved as int64

	dataOffset = nextBlockIDOffset + blockIDSize //This is the offset where the actual data is stored

	MinBlockSize = dataOffset //At least one byte per block
)

type Block struct {
	*os.File
	offset    int64
	maxOffset int64

	size      int64
	blockNum  int64
	nextBlock int64

	blockCipher cipher.Block
	iv          []byte
}

func NewBlock(f *os.File, key []byte, off, num, size int64) (*Block, error) {
	if size < MinBlockSize {
		return nil, fmt.Errorf("Block size %d too small, must be at least %d", size, MinBlockSize)
	}
	bc, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	offset := size*num + off
	return &Block{File: f, offset: offset, blockNum: num, maxOffset: offset + size, size: size, blockCipher: bc}, nil
}

func (b *Block) GetDataSize() int64 {
	return b.size - dataOffset
}

func (b *Block) getCTR(off int64) (cipher.Stream, error) {
	if err := b.initIV(); err != nil {
		return nil, err
	}
	keyBlock := off / int64(len(b.iv))
	ivCopy := make([]byte, len(b.iv))
	copy(ivCopy, b.iv)
	IncrementIV(ivCopy, keyBlock)
	var blockSize = b.blockCipher.BlockSize()
	_ = blockSize
	return cipher.NewCTR(b.blockCipher, ivCopy), nil
}

func (b *Block) getIV() ([]byte, error) {
	iv := make([]byte, ivSize)
	_, err := b.File.ReadAt(iv, b.offset+ivOffset)
	return iv, err
}

func (b *Block) initIV() error {
	if b.iv != nil {
		return nil
	}
	iv, err := b.getIV()
	if err != nil {
		return err
	}
	b.iv = iv
	return nil
}

func (b *Block) Validate() error {
	magic := make([]byte, blockMagicSize)
	_, err := b.readAt(magic, blockMagicOffset)
	if err != nil {
		return err
	}
	if !bytes.Equal(magic, blockStartingMagic) {
		return errors.New("invalid block")
	}
	return nil
}

func (b *Block) Write(nextBlockID int64) error {
	if b.iv == nil {
		b.iv = make([]byte, ivSize)
		_, err := rand.Read(b.iv)
		if err != nil {
			return err
		}
	}
	_, err := b.File.WriteAt(b.iv, b.offset+ivOffset)
	if err != nil {
		return fmt.Errorf("error writing iv: %v", err)
	}
	_, err = b.writeAt(blockStartingMagic, blockMagicOffset)
	if err != nil {
		return fmt.Errorf("error writing block magic: %v", err)
	}
	return b.SetNextBlockID(nextBlockID)
}

func (b *Block) GetNextBlockID() (int64, error) {
	idBytes := make([]byte, blockIDSize)
	_, err := b.readAt(idBytes, nextBlockIDOffset)
	if err != nil {
		return 0, err
	}
	b.nextBlock = int64(binary.LittleEndian.Uint64(idBytes))
	return b.nextBlock, nil
}

func (b *Block) SetNextBlockID(id int64) error {
	idBytes := make([]byte, blockIDSize)
	binary.LittleEndian.PutUint64(idBytes, uint64(id))
	_, err := b.writeAt(idBytes, nextBlockIDOffset)
	if err != nil {
		return fmt.Errorf("error writing block id: %v", err)
	}
	b.nextBlock = id
	return nil
}

func (b *Block) getActualOffsetAndSize(requestedOffset int64, requestedSize int) (int64, int) {
	actualOff := b.offset + requestedOffset
	actualSize := requestedSize
	if (actualOff + int64(requestedSize)) > b.maxOffset {
		actualSize = int(b.maxOffset - actualOff)
	}
	return actualOff, actualSize
}

func (b *Block) readAt(p []byte, off int64) (int, error) {
	actualOffset, actualSize := b.getActualOffsetAndSize(off, len(p))
	fromStart := actualOffset % int64(b.blockCipher.BlockSize())
	if fromStart != 0 {
		newP := make([]byte, actualSize+int(fromStart))
		_, err := b.readAt(newP, off-fromStart)
		if err != nil {
			return 0, err
		}
		n := copy(p, newP[fromStart:])
		return n, nil
	}
	ctr, err := b.getCTR(actualOffset)
	if err != nil {
		return 0, err
	}
	buf := make([]byte, actualSize)
	_, err = b.File.ReadAt(buf, actualOffset)
	if err != nil {
		return 0, err
	}
	ctr.XORKeyStream(p, buf)
	if len(p) != actualSize {
		return actualSize, io.EOF
	}
	return len(p), nil
}

func (b *Block) ReadAt(p []byte, off int64) (int, error) {
	return b.readAt(p, dataOffset+off)
}

func (b *Block) writeAt(p []byte, off int64) (int, error) {
	actualOffset, actualSize := b.getActualOffsetAndSize(off, len(p))
	fromStart := actualOffset % int64(b.blockCipher.BlockSize())
	if fromStart != 0 {
		old := make([]byte, fromStart)
		_, err := b.readAt(old, off-fromStart)
		if err != nil {
			return 0, err
		}
		newP := make([]byte, actualSize+int(fromStart))
		copy(newP, old)
		copy(newP[fromStart:], p)
		n, err := b.writeAt(newP, off-fromStart)
		return n - int(fromStart), err
	}
	ctr, err := b.getCTR(actualOffset)
	if err != nil {
		return 0, err
	}
	buf := make([]byte, actualSize)
	ctr.XORKeyStream(buf, p[:actualSize])
	n, err := b.File.WriteAt(buf, actualOffset)
	if err != nil {
		return n, err
	}
	if actualSize != len(p) {
		return actualSize, io.ErrShortWrite
	}
	return len(p), nil
}

func (b *Block) WriteAt(p []byte, off int64) (int, error) {
	return b.writeAt(p, off+dataOffset)
}
