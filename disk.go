package rubberhose

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"math/big"
	"os"

	"golang.org/x/crypto/scrypt"
)

var StartingMagic = []byte{53, 83, 156, 194}

const (
	diskMagicOffset = 0
	diskMagicSize   = 4 //Same as len(StartingMagic)
	blockSizeOffset = diskMagicOffset + diskMagicSize
	blockSizeSize   = 8
	saltOffset      = blockSizeOffset + blockSizeSize
	saltSize        = 8
	diskDataOffset  = saltOffset + saltSize
)

type Disk struct {
	*os.File
	usedBlocks map[int64]struct{}
}

func NewDisk(path string) (*Disk, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0755)
	if err != nil {
		return nil, err
	}
	return NewDiskFromFile(f), nil
}

func NewDiskFromFile(f *os.File) Disk {
	return Disk{File: f, usedBlocks: make(map[int64]struct{})}
}

func (d Disk) Verify() error {
	magic := make([]byte, diskMagicSize)
	_, err := d.ReadAt(magic, diskMagicOffset)
	if err != nil {
		return err
	}
	if !bytes.Equal(magic, StartingMagic) {
		return errors.New("invalid disk")
	}
	return nil
}

func (d Disk) GetBlockSize() (int64, error) {
	bs := make([]byte, blockSizeSize)
	_, err := d.ReadAt(bs, blockSizeOffset)
	if err != nil {
		return 0, err
	}
	return int64(binary.LittleEndian.Uint64(bs)), nil
}

func (d Disk) writeBlockSize(blockSize int64) error {
	bs := make([]byte, blockSizeSize)
	binary.LittleEndian.PutUint64(bs, uint64(blockSize))
	_, err := d.WriteAt(bs, blockSizeOffset)
	return err
}

func (d Disk) getSalt() ([]byte, error) {
	p := make([]byte, saltSize)
	_, err := d.ReadAt(p[:saltSize], saltOffset)
	return p, err
}

func (d Disk) Write(blockSize, blockCount int64) error {
	_, err := d.WriteAt(StartingMagic, diskMagicOffset)
	if err != nil {
		return err
	}
	err = d.writeBlockSize(blockSize)
	if err != nil {
		return err
	}
	salt := make([]byte, saltSize)
	_, err = rand.Read(salt)
	if err != nil {
		return err
	}
	_, err = d.WriteAt(salt, saltOffset)
	if err != nil {
		return err
	}
	_, err = d.Seek(diskDataOffset, 0)
	if err != nil {
		return err
	}
	_, err = io.CopyN(d.File, rand.Reader, blockCount*blockSize)
	return err
}

func (d Disk) GetBlockCount() (int64, error) {
	info, err := d.Stat()
	if err != nil {
		return 0, err
	}
	blockSize, err := d.GetBlockSize()
	if err != nil {
		return 0, err
	}
	return (info.Size()-dataOffset)/blockSize + 1, nil
}

func (d Disk) GetBlock(blockNum int64, key []byte) (*Block, error) {
	blockSize, err := d.GetBlockSize()
	if err != nil {
		return nil, err
	}
	return NewBlock(&d, key, dataOffset, blockNum, blockSize)
}

func (d Disk) getKey(password string) ([]byte, error) {
	salt, err := d.getSalt()
	if err != nil {
		return nil, err
	}
	return scrypt.Key([]byte(password), salt, 32768, 8, 1, 32)
}

func (d Disk) GetPartition(password string) (*Partition, error) {
	key, err := d.getKey(password)
	if err != nil {
		return nil, err
	}
	var blocks []*Block
	blockCount, err := d.GetBlockCount()
	if err != nil {
		return nil, err
	}
	for i := int64(0); i < blockCount; i++ {
		b, err := d.GetBlock(i, key)
		if err != nil {
			return nil, err
		}
		if b.Validate() == nil {
			blocks = append(blocks, b)
			d.usedBlocks[i] = struct{}{}
		}
	}
	if len(blocks) == 0 {
		return nil, errors.New("no partition with that password")
	}
	blockSize, err := d.GetBlockSize()
	if err != nil {
		return nil, err
	}
	part := &Partition{blockSize: blockSize, blocks: blocks, Disk: &d}
	err = part.orderBlocks()
	return part, err
}

func (d Disk) WritePartition(password string, blockCount int64) (*Partition, error) {
	key, err := d.getKey(password)
	if err != nil {
		return nil, err
	}
	blocksOnDisk, err := d.GetBlockCount()
	if err != nil {
		return nil, err
	}
	bigBlocksOnDisk := big.NewInt(blocksOnDisk)
	var lastBlock *Block
	blocks := make([]*Block, 0, blockCount)
	for i := int64(0); i < blockCount; i++ {
		var blockID int64
		for true {
			if len(d.usedBlocks) == int(blocksOnDisk) {
				return nil, errors.New("all blocks allocated")
			}
			r, err := rand.Int(rand.Reader, bigBlocksOnDisk)
			if err != nil {
				log.Fatal(err)
			}
			blockID = r.Int64()
			_, used := d.usedBlocks[blockID]
			if !used {
				break
			}
		}
		d.usedBlocks[blockID] = struct{}{}
		block, err := d.GetBlock(blockID, key)
		if err != nil {
			return nil, err
		}
		if lastBlock != nil {
			err = lastBlock.Write(blockID)
			if err != nil {
				return nil, err
			}
		}
		lastBlock = block
		blocks = append(blocks, block)
	}
	err = blocks[len(blocks)-1].Write(-1)
	if err != nil {
		return nil, err
	}
	blockSize, err := d.GetBlockSize()
	if err != nil {
		return nil, err
	}
	return &Partition{blockSize: blockSize, blocks: blocks, Disk: &d}, nil
}
