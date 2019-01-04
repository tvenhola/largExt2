package ext2fs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func (d *Device) Offsets(blockOffset uint32) (dirIdx int64, indIdx int64, dindIdx int64, tindIdx int64) {
	dirIdx = -1
	indIdx = -1
	dindIdx = -1
	tindIdx = -1

	//Direct Blocks
	if blockOffset < EXT2_NDIR_BLOCKS {
		dirIdx = int64(blockOffset)
		return
	}

	maxBlocks := d.BlockSize / 4
	blocks := blockOffset - EXT2_NDIR_BLOCKS

	//Indirect Blocks
	if blocks < maxBlocks {
		indIdx = int64(blocks)
		return
	}

	blocks -= maxBlocks

	//Double Indirect Blocks
	if blocks < maxBlocks*maxBlocks {
		dindIdx = int64(blocks / maxBlocks)
		indIdx = int64(blocks - uint32(dindIdx)*maxBlocks)
		return
	}

	blocks -= maxBlocks * maxBlocks

	//Triple Indirect Blocks
	if blocks < maxBlocks*maxBlocks*maxBlocks {
		tindIdx = int64(blocks / (maxBlocks * maxBlocks))
		dindIdx = int64((blocks / maxBlocks) - (uint32(tindIdx) * maxBlocks))
		indIdx = int64(blocks - uint32(dindIdx)*maxBlocks - uint32(tindIdx)*maxBlocks*maxBlocks)
		return
	}

	return
}

func (d *Device) ExtractBlock(block uint32, index int64) (uint32, error) {
	buffer := make([]byte, 4)

	if block == EXT2_NULL_BLOCK {
		return block, errors.New("Inode block offset out of bounds")
	}

	if _, err := d.file.ReadAt(buffer, d.blockOffset(block)+index*4); err != nil {
		return EXT2_NULL_BLOCK, err
	}

	return binary.LittleEndian.Uint32(buffer), nil
}

func (d *Device) DataBlock(inode *Inode, blockOffset uint32) (uint32, error) {
	dirIdx, indIdx, dindIdx, tindIdx := d.Offsets(blockOffset)

	if dirIdx != -1 {
		block := inode.Block[dirIdx]

		if block == EXT2_NULL_BLOCK {
			return EXT2_NULL_BLOCK, errors.New(fmt.Sprintf("Inode block offset %d out of bounds", blockOffset))
		}

		return block, nil
	}

	if tindIdx != -1 {
		block, err := d.ExtractBlock(inode.Block[EXT2_TIND_BLOCK], tindIdx)
		if err != nil {
			return block, err
		}

		block, err = d.ExtractBlock(block, dindIdx)
		if err != nil {
			return block, err
		}

		return d.ExtractBlock(block, indIdx)
	}

	if dindIdx != -1 {
		block, err := d.ExtractBlock(inode.Block[EXT2_DIND_BLOCK], dindIdx)
		if err != nil {
			return block, err
		}

		return d.ExtractBlock(block, indIdx)
	}

	if indIdx != -1 {
		return d.ExtractBlock(inode.Block[EXT2_IND_BLOCK], indIdx)
	}

	return EXT2_NULL_BLOCK, errors.New("No block offsets given")
}

func (d *Device) CreateDataBlock(inodeNo uint32) (uint32, error) {
	inode, err := d.NewInode(inodeNo)
	if err != nil {
		return EXT2_NULL_BLOCK, err
	}

	groupNo := (inodeNo - 1) / d.InodesPerGroup
	inodeIdx := (inodeNo - 1) % d.InodesPerGroup
	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return EXT2_NULL_BLOCK, err
	}

	blockOffset := inode.Size / d.BlockSize
	dirIdx, indIdx, dindIdx, tindIdx := d.Offsets(blockOffset)

	//Direct Blocks
	if dirIdx != -1 {
		newBlock, err := d.AllocBlock(groupNo)
		if err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if _, err := d.seek(d.inodeOffset(group.InodeTable, inodeIdx) + I_BLOCK + 4*dirIdx); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if err := binary.Write(d.file, binary.LittleEndian, newBlock); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		return newBlock, nil
	}

	//Triple Indirect Blocks
	if tindIdx != -1 {
		tindBlock := inode.Block[EXT2_TIND_BLOCK]

		if tindBlock == EXT2_NULL_BLOCK {
			//New Triple Indirect Block
			tindBlock, err = d.AllocBlock(groupNo)
			if err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if _, err := d.seek(d.inodeOffset(group.InodeTable, inodeIdx) + I_BLOCK + 4*EXT2_TIND_BLOCK); err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if err := binary.Write(d.file, binary.LittleEndian, tindBlock); err != nil {
				return EXT2_NULL_BLOCK, err
			}
		}

		dindBlock, err := d.ExtractBlock(tindBlock, tindIdx)
		if err != nil {
			return dindBlock, err
		}

		if dindBlock == EXT2_NULL_BLOCK {
			//New Double Indirect Block
			dindBlock, err = d.AllocBlock(groupNo)
			if err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if _, err := d.seek(d.blockOffset(inode.Block[EXT2_TIND_BLOCK]) + 4*tindIdx); err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if err := binary.Write(d.file, binary.LittleEndian, dindBlock); err != nil {
				return EXT2_NULL_BLOCK, err
			}
		}

		indBlock, err := d.ExtractBlock(dindBlock, dindIdx)
		if err != nil {
			return indBlock, err
		}

		if indBlock == EXT2_NULL_BLOCK {
			//New Indirect Block
			indBlock, err := d.AllocBlock(groupNo)
			if err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if _, err := d.seek(d.blockOffset(dindBlock) + 4*dindIdx); err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if err := binary.Write(d.file, binary.LittleEndian, indBlock); err != nil {
				return EXT2_NULL_BLOCK, err
			}
		}

		//New Direct Block
		newBlock, err := d.AllocBlock(groupNo)
		if err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if _, err := d.seek(d.blockOffset(indBlock) + 4*indIdx); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if err := binary.Write(d.file, binary.LittleEndian, newBlock); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		return newBlock, nil
	}

	//Double Indirect Blocks
	if dindIdx != -1 {
		dindBlock := inode.Block[EXT2_DIND_BLOCK]

		if dindBlock == EXT2_NULL_BLOCK {
			//New Double Indirect Block
			dindBlock, err = d.AllocBlock(groupNo)
			if err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if _, err := d.seek(d.inodeOffset(group.InodeTable, inodeIdx) + I_BLOCK + 4*EXT2_DIND_BLOCK); err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if err := binary.Write(d.file, binary.LittleEndian, dindBlock); err != nil {
				return EXT2_NULL_BLOCK, err
			}
		}

		indBlock, err := d.ExtractBlock(dindBlock, dindIdx)
		if err != nil {
			return indBlock, err
		}

		if indBlock == EXT2_NULL_BLOCK {
			//New Indirect Block
			indBlock, err := d.AllocBlock(groupNo)
			if err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if _, err := d.seek(d.blockOffset(dindBlock) + 4*dindIdx); err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if err := binary.Write(d.file, binary.LittleEndian, indBlock); err != nil {
				return EXT2_NULL_BLOCK, err
			}
		}

		//New Direct Block
		newBlock, err := d.AllocBlock(groupNo)
		if err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if _, err := d.seek(d.blockOffset(indBlock) + 4*indIdx); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if err := binary.Write(d.file, binary.LittleEndian, newBlock); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		return newBlock, nil
	}

	if indIdx != -1 {
		indBlock := inode.Block[EXT2_IND_BLOCK]

		if indBlock == EXT2_NULL_BLOCK {
			//New Indirect Block
			indBlock, err = d.AllocBlock(groupNo)
			if err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if _, err := d.seek(d.inodeOffset(group.InodeTable, inodeIdx) + I_BLOCK + 4*EXT2_IND_BLOCK); err != nil {
				return EXT2_NULL_BLOCK, err
			}

			if err := binary.Write(d.file, binary.LittleEndian, indBlock); err != nil {
				return EXT2_NULL_BLOCK, err
			}
		}

		//New Direct Block
		newBlock, err := d.AllocBlock(groupNo)
		if err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if _, err := d.seek(d.blockOffset(indBlock) + 4*indIdx); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		if err := binary.Write(d.file, binary.LittleEndian, newBlock); err != nil {
			return EXT2_NULL_BLOCK, err
		}

		return newBlock, nil
	}

	return EXT2_NULL_BLOCK, errors.New("No block offsets given")
}

//TODO: Refactor this function
func (d *Device) ReadData(inode *Inode, b []byte, off int64) (n int, err error) {
	size := len(b)
	blockOffset := uint32(off / int64(d.BlockSize))
	innerOffset := off % int64(d.BlockSize)
	blocks := 1 + (uint32((off+int64(size))/int64(d.BlockSize)) - blockOffset)

	if len(b) == 0 {
		return 0, nil
	}

	if off >= int64(inode.Size) {
		return 0, io.EOF
	}

	blockNo, err := d.DataBlock(inode, blockOffset)
	if err != nil {
		return n, err
	}

	read := int(int64(d.BlockSize) - innerOffset)
	if read > size {
		read = size
	}

	if _, err := d.file.ReadAt(b[n:n+read], d.blockOffset(blockNo)+innerOffset); err != nil {
		return n, err
	}

	n += read

	if int64(n)+off >= int64(inode.Size) {
		return n, io.EOF
	}

	for i := uint32(1); i < blocks; i++ {
		blockNo, err := d.DataBlock(inode, blockOffset+i)
		if err != nil {
			return n, err
		}

		if blockNo == EXT2_NULL_BLOCK {
			return n, io.EOF
		}

		read = size - n
		if read > int(d.BlockSize) {
			read = int(d.BlockSize)
		}

		if _, err := d.file.ReadAt(b[n:n+read], d.blockOffset(blockNo)); err != nil {
			return n, err
		}

		n += read

		if int64(n)+off >= int64(inode.Size) {
			return n, io.EOF
		}
	}

	return n, nil
}

func (d *Device) WriteData(inode *Inode, b []byte, off int64) (n int, err error) {
	size := len(b)
	blockOffset := uint32(off / int64(d.BlockSize))
	innerOffset := off % int64(d.BlockSize)
	blocks := 1 + (uint32((off+int64(size))/int64(d.BlockSize)) - blockOffset)

	if len(b) == 0 {
		return 0, nil
	}

	blockNo, err := d.DataBlock(inode, blockOffset)
	if err != nil {
		return n, err
	}

	write := int(int64(d.BlockSize) - innerOffset)
	if write > size {
		write = size
	}

	if _, err := d.file.WriteAt(b[:write], d.blockOffset(blockNo)+innerOffset); err != nil {
		return n, err
	}

	n += write
	b = b[write:]

	for i := uint32(1); i < blocks; i++ {
		blockNo, err := d.DataBlock(inode, blockOffset+i)
		if err != nil {
			return n, err
		}

		if blockNo == EXT2_NULL_BLOCK {
			return n, nil
		}

		write = size - n
		if write > int(d.BlockSize) {
			write = int(d.BlockSize)
		}

		if _, err := d.file.WriteAt(b[:write], d.blockOffset(blockNo)); err != nil {
			return n, err
		}

		n += write
		b = b[write:]
	}

	return n, nil
}
