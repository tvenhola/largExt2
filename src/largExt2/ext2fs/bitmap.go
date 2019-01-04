package ext2fs

import (
	"encoding/binary"
	"errors"
)

type Bitmap []byte

func (d *Device) NewBitmap(size uint32, blockOffset int64) (Bitmap, error) {
	bmp := make([]byte, size)

	if _, err := d.file.ReadAt(bmp, blockOffset); err != nil {
		return nil, err
	}

	return bmp, nil
}

func (bmp Bitmap) IsFree(index uint32) bool {
	return (bmp[index/8] & (1 << (index % 8))) == 0
}

func (bmp Bitmap) Alloc(index uint32) {
	bmp[index/8] |= (1 << (index % 8))
}

func (bmp Bitmap) Free(index uint32) {
	bmp[index/8] &^= (1 << (index % 8))
}

func (bmp Bitmap) FindFree() int {
	for i, bits := uint32(0), uint32(len(bmp)*8); i < bits; i++ {
		if bmp.IsFree(i) {
			return int(i)
		}
	}

	return -1
}

func (d *Device) NewInodeBitmap(groupNo uint32) (Bitmap, error) {
	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return nil, err
	}

	return d.NewBitmap(d.InodesPerGroup/8, d.blockOffset(group.InodeBitmap))
}

func (d *Device) next(bitmap func(uint32) (Bitmap, error), groupNo uint32) (Bitmap, int, uint32, error) {
	bmp, err := bitmap(groupNo)
	if err != nil {
		return bmp, 0, groupNo, err
	}

	index := bmp.FindFree()
	if index != -1 {
		return bmp, index, groupNo, nil
	}

	for i := (groupNo + 1) % d.BlockGroupsCount; i != groupNo; i = (i + 1) % d.BlockGroupsCount {
		bmp, err := bitmap(i)
		if err != nil {
			return bmp, 0, i, err
		}

		index = bmp.FindFree()
		if index != -1 {
			return bmp, index, i, nil
		}
	}

	return nil, -1, 0, nil
}

func (d *Device) nextInode(groupNo uint32) (Bitmap, int, uint32, error) {
	return d.next(d.NewInodeBitmap, groupNo)
}

func (d *Device) allocInode(groupNo uint32) (uint32, error) {
	bmp, index, groupNo, err := d.nextInode(groupNo)
	if err != nil {
		return EXT2_NULL_INO, err
	}

	if index == -1 {
		return EXT2_NULL_INO, nil
	}

	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return EXT2_NULL_INO, err
	}

	bmp.Alloc(uint32(index))

	//Update Group Descriptor Inode Bitmap
	if _, err := d.seek(d.blockOffset(group.InodeBitmap) + int64(index/8)); err != nil {
		return EXT2_NULL_INO, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, bmp[index/8]); err != nil {
		return EXT2_NULL_INO, err
	}

	//Update Group Descriptor Free Inodes Count
	if _, err := d.seek(d.groupDescriptorOffset(groupNo) + BG_FREE_INODES_COUNT); err != nil {
		return EXT2_NULL_INO, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, group.FreeInodesCount-1); err != nil {
		return EXT2_NULL_INO, err
	}

	return ((d.InodesPerGroup * groupNo) + uint32(index) + 1), nil
}

func (d *Device) AllocInode(groupNo uint32) (uint32, error) {
	super, err := d.NewSuperBlock()
	if err != nil {
		return EXT2_NULL_INO, err
	}

	if super.FreeInodesCount == 0 {
		return EXT2_NULL_INO, errors.New("Inodes disk limit reached")
	}

	inodeNo, err := d.allocInode(groupNo)
	if err != nil {
		return EXT2_NULL_INO, nil
	}

	if _, err := d.seek(BASE_OFFSET + S_FREE_INODES_COUNT); err != nil {
		return EXT2_NULL_INO, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, super.FreeInodesCount-1); err != nil {
		return EXT2_NULL_INO, err
	}

	d.file.Sync()

	return inodeNo, nil
}

func (d *Device) NewBlockBitmap(groupNo uint32) (Bitmap, error) {
	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return nil, err
	}

	return d.NewBitmap(d.BlocksPerGroup/8, d.blockOffset(group.BlockBitmap))
}

func (d *Device) nextBlock(groupNo uint32) (Bitmap, int, uint32, error) {
	return d.next(d.NewBlockBitmap, groupNo)
}

func (d *Device) allocBlock(groupNo uint32) (uint32, error) {
	bmp, index, groupNo, err := d.nextBlock(groupNo)
	if err != nil {
		return EXT2_NULL_BLOCK, err
	}

	if index == -1 {
		return EXT2_NULL_BLOCK, nil
	}

	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return EXT2_NULL_BLOCK, err
	}

	bmp.Alloc(uint32(index))

	//Update Group Descriptor Block Bitmap
	if _, err := d.seek(d.blockOffset(group.BlockBitmap) + int64(index/8)); err != nil {
		return EXT2_NULL_BLOCK, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, bmp[index/8]); err != nil {
		return EXT2_NULL_BLOCK, err
	}

	//Update Group Descriptor Free Blocks Count
	if _, err := d.seek(d.groupDescriptorOffset(groupNo) + BG_FREE_BLOCKS_COUNT); err != nil {
		return EXT2_NULL_BLOCK, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, group.FreeBlocksCount-1); err != nil {
		return EXT2_NULL_BLOCK, err
	}

	return (d.BlocksPerGroup * groupNo) + uint32(index) + 1, nil
}

func (d *Device) AllocBlock(groupNo uint32) (uint32, error) {
	super, err := d.NewSuperBlock()
	if err != nil {
		return EXT2_NULL_BLOCK, err
	}

	if super.FreeBlocksCount == 0 {
		return EXT2_NULL_BLOCK, errors.New("Blocks disk limit reached")
	}

	blockNo, err := d.allocBlock(groupNo)
	if err != nil {
		return EXT2_NULL_BLOCK, nil
	}

	if _, err := d.seek(BASE_OFFSET + S_FREE_BLOCKS_COUNT); err != nil {
		return EXT2_NULL_BLOCK, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, super.FreeBlocksCount-1); err != nil {
		return EXT2_NULL_BLOCK, err
	}

	d.file.Sync()

	return blockNo, nil
}
