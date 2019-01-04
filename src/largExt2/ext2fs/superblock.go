package ext2fs

import (
	"encoding/binary"
	"errors"
)

type SuperBlock struct {
	InodesCount     uint32
	BlocksCount     uint32
	RBlocksCount    uint32
	FreeBlocksCount uint32
	FreeInodesCount uint32
	FirstDataBlock  uint32
	LogBlockSize    uint32
	LogFragSize     int32
	BlocksPerGroup  uint32
	FragsPerGroup   uint32
	InodesPerGroup  uint32
	MTime           uint32
	WTime           uint32
	MntCount        uint16
	MaxMntCount     int16
	Magic           uint16
	State           uint16
	Errors          uint16
	MinorRevLevel   uint16
	LastCheck       uint32
	CheckInterval   uint32
	CreatorOS       uint32
	RevLevel        uint32
	DefResUID       uint16
	DefResGID       uint16

	/*
		EXT2_DYNAMIC_REV superblocks only
	*/
	FirstIno             uint32
	InodeSize            uint16
	BlockGroupNr         uint16
	FeatureCompat        uint32
	FeatureIncompat      uint32
	FeatureRoCompat      uint32
	UUID                 [16]uint8
	VolumeName           [16]byte
	LastMounted          [64]byte
	AlgorithmUsageBitmap uint32

	/*
		Performance hints. Directory preallocation should only
		happen if the EXT2_COMPAT_PREALLOC flag is on.
	*/
	PreallocBlocks    uint8
	PreallocDirBlocks uint8
	Padding1          uint16
	Reserved          [204]uint32
}

func (d *Device) NewSuperBlock() (*SuperBlock, error) {
	_, err := d.seek(BASE_OFFSET)
	if err != nil {
		return nil, err
	}

	super := &SuperBlock{}
	if err := binary.Read(d.file, binary.LittleEndian, super); err != nil {
		return nil, err
	}

	if super.Magic != EXT2_SUPER_MAGIC {
		return nil, errors.New("Not an ext2 filesystem")
	}

	return super, nil
}
