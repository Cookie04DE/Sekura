package rubberhose

import (
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/dop251/buse"
)

type Partition struct {
	*Disk
	*ExposedPartition
	blockSize int64
	key       []byte
	blocks    []*Block
}

type ExposedPartition struct {
	*buse.Device
	Path string
}

func NewPartition(blockSize int64, blocks []*Block) Partition {
	return Partition{blockSize: blockSize, blocks: blocks}
}

func (par *Partition) ReadAt(p []byte, off int64) (int, error) {
	blockNum := off / par.blockSize
	blockOff := off % par.blockSize
	if blockNum > int64(len(par.blocks)) {
		return 0, io.EOF
	}
	originalLength := len(p)
	for len(p) > 0 {
		if int(blockNum) >= len(par.blocks) {
			return originalLength - len(p), io.EOF
		}
		read, err := par.blocks[blockNum].ReadAt(p, blockOff)
		if err != io.EOF {
			return originalLength - len(p), err
		}
		p = p[read:]
		blockOff = 0
		blockNum++
	}
	return originalLength, nil
}

func (par *Partition) WriteAt(p []byte, off int64) (int, error) {
	blockNum := off / par.blockSize
	blockOff := off % par.blockSize
	if blockNum > int64(len(par.blocks)) {
		return 0, io.EOF
	}
	originalLength := len(p)
	for len(p) > 0 {
		if int(blockNum) >= len(par.blocks) {
			return originalLength - len(p), io.EOF
		}
		written, err := par.blocks[blockNum].WriteAt(p, blockOff)
		p = p[written:]
		if err != nil && err != io.ErrShortWrite {
			return originalLength - len(p), err
		}
		blockOff = 0
		blockNum++
	}
	return originalLength, nil
}

func (par *Partition) GetBlockCount() int {
	return len(par.blocks)
}

func (par *Partition) GetDataSize() int64 {
	var size int64
	for _, b := range par.blocks {
		size += b.GetDataSize()
	}
	return size
}

var ErrInvalidBlockStructure = errors.New("invalid block structure")

func (par *Partition) orderBlocks() error {
	blocks := par.blocks
	finished := make([]*Block, 0, len(par.blocks))
	for len(blocks) != 0 {
		blockToRemove := -1
		for i, b := range blocks {
			nextBlockNum, err := b.GetNextBlockID()
			if err != nil {
				return err
			}
			if len(finished) == 0 {
				if nextBlockNum != -1 {
					continue
				}
				finished = append(finished, b)
				blockToRemove = i
				break
			}
			lastBlockNum := finished[len(finished)-1].num
			if nextBlockNum == lastBlockNum {
				finished = append(finished, b)
				blockToRemove = i
				break
			}
		}
		if blockToRemove == -1 {
			return ErrInvalidBlockStructure
		}
		blocks[len(blocks)-1], blocks[blockToRemove] = blocks[blockToRemove], blocks[len(blocks)-1]
		blocks = blocks[:len(blocks)-1]
	}
	for i, j := 0, len(finished)-1; i < j; i, j = i+1, j-1 {
		finished[i], finished[j] = finished[j], finished[i]
	}
	copy(par.blocks, finished)
	return nil
}

func (par *Partition) Close() error {
	return par.Sync()
}

var counter int

func (par *Partition) Expose() (string, *buse.Device) {
	for {
		path := fmt.Sprintf("/dev/nbd%d", counter)
		counter++
		bd, err := par.ExposePath(path)
		if err != nil {
			continue
		}
		return path, bd
	}
}

func (par *Partition) ExposePath(path string) (*buse.Device, error) {
	if ep := par.ExposedPartition; ep != nil {
		ep.Device.Disconnect()
		par.ExposedPartition = nil
	}
	bd, err := buse.NewDevice(path, par.GetDataSize(), par)
	if err != nil {
		return nil, err
	}
	go func() {
		err := bd.Run()
		if err != nil {
			log.Fatal("Error running buse device: ", err)
		}
	}()
	par.ExposedPartition = &ExposedPartition{Path: path, Device: bd}
	return bd, nil
}

func (par Partition) Delete() error {
	for _, b := range par.blocks {
		err := b.Delete()
		if err != nil {
			return err
		}
		delete(par.Disk.usedBlocks, b.num)
	}
	return par.Sync()
}

func (par *Partition) Resize(blockCount int) error {
	delta := blockCount - len(par.blocks)
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		lastBlock := par.blocks[len(par.blocks)-1]
		for i := 0; i < delta; i++ {
			block, err := par.Disk.allocateBlock(par.key)
			if err != nil {
				return err
			}
			if err := lastBlock.Write(block.num); err != nil {
				return err
			}
			par.blocks = append(par.blocks, block)
		}
	} else {
		off := len(par.blocks) + delta
		toDelete := par.blocks[off:]
		par.blocks = par.blocks[:off]
		for _, b := range toDelete {
			if err := b.Delete(); err != nil {
				return err
			}
		}
	}
	return par.blocks[len(par.blocks)-1].Write(-1)
}
