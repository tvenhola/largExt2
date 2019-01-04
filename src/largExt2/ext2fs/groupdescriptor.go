package ext2fs

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type GroupDescriptor struct {
	BlockBitmap     uint32
	InodeBitmap     uint32
	InodeTable      uint32
	FreeBlocksCount uint16
	FreeInodesCount uint16
	UsedDirsCount   uint16
	Pad             uint16
	Reserved        [3]uint32
}

func (d *Device) NewGroupDescriptor(index uint32) (*GroupDescriptor, error) {
	if index >= d.BlockGroupsCount {
		return nil, errors.New(fmt.Sprintf("Group descriptor index %d out of bounds", index))
	}

	if _, err := d.seek(d.groupDescriptorOffset(index)); err != nil {
		return nil, err
	}

	group := &GroupDescriptor{}
	if err := binary.Read(d.file, binary.LittleEndian, group); err != nil {
		return nil, err
	}

	return group, nil
}
