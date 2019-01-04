package ext2fs

import (
	"errors"
	"os"
)

type Device struct {
	file                *os.File
	BlockSize           uint32
	InodeSize           uint16
	BlockGroupsCount    uint32
	BlocksCount         uint32
	InodesCount         uint32
	BlocksPerGroup      uint32
	InodesPerGroup      uint32
	GroupDescTableBlock uint32
	FirstIno            uint32
}

func NewDevice(path string) (*Device, error) {
	file, err := os.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}

	device := &Device{file: file}
	super, err := device.NewSuperBlock()
	if err != nil {
		return nil, err
	}

	if super.RevLevel < EXT2_DYNAMIC_REV {
		return nil, errors.New("ext2 filesystem must be revision 1 or higher")
	}

	device.BlockSize = EXT2_DEFAULT_BLOCK_SIZE << super.LogBlockSize
	device.BlockGroupsCount = 1 + ((super.BlocksCount - 1) / super.BlocksPerGroup)
	device.BlocksCount = super.BlocksCount
	device.InodesCount = super.InodesCount
	device.BlocksPerGroup = super.BlocksPerGroup
	device.InodesPerGroup = super.InodesPerGroup
	device.InodeSize = super.InodeSize
	device.GroupDescTableBlock = super.FirstDataBlock + 1
	device.FirstIno = super.FirstIno

	return device, nil
}

func (d *Device) Close() error {
	return d.file.Close()
}

func (d *Device) blockOffset(blockNo uint32) int64 {
	return int64(blockNo) * int64(d.BlockSize)
}

func (d *Device) inodeOffset(inodeTable uint32, index uint32) int64 {
	return d.blockOffset(inodeTable) + int64(index)*int64(d.InodeSize)
}

func (d *Device) groupDescriptorOffset(index uint32) int64 {
	return d.blockOffset(d.GroupDescTableBlock) + int64(index)*EXT2_GROUP_DESC_SIZE
}

func (d *Device) seek(offset int64) (int64, error) {
	return d.file.Seek(offset, os.SEEK_SET)
}

func (d *Device) Sync() error {
	return d.file.Sync()
}

func (d *Device) File() *os.File {
	return d.file
}
