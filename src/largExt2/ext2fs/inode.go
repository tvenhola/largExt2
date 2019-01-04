package ext2fs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
)

type linux1 struct {
	Reserved1 uint32
}

type hurd1 struct {
	Translator uint32
}

type masix1 struct {
	Reserved1 uint32
}

type linux2 struct {
	Frag      uint8
	FSize     uint8
	Pad1      uint16
	UIDHigh   uint16
	GIDHigh   uint16
	Reserved2 uint32
}

type hurd2 struct {
	Frag     uint8
	FSize    uint8
	ModeHigh uint16
	UIDHigh  uint16
	GIDHigh  uint16
	Author   uint32
}

type masix2 struct {
	Frag     uint8
	FSize    uint8
	Pad1     uint16
	Reserved [2]uint32
}

type Inode struct {
	Mode       uint16
	UID        uint16
	Size       uint32
	ATime      uint32
	CTime      uint32
	MTime      uint32
	DTime      uint32
	GID        uint16
	LinksCount uint16
	Blocks     uint32
	Flags      uint32
	Osd1       [4]byte
	Block      [EXT2_N_BLOCKS]uint32
	Generation uint32
	FileACL    uint32
	DirACL     uint32
	FAddr      uint32
	Osd2       [12]byte
}

func (i *Inode) Linux1() *linux1 {
	linux1 := &linux1{}
	binary.Read(bytes.NewBuffer(i.Osd1[:]), binary.LittleEndian, linux1)
	return linux1
}

func (i *Inode) Hurd1() *hurd1 {
	hurd1 := &hurd1{}
	binary.Read(bytes.NewBuffer(i.Osd1[:]), binary.LittleEndian, hurd1)
	return hurd1
}

func (i *Inode) Masix1() *masix1 {
	masix1 := &masix1{}
	binary.Read(bytes.NewBuffer(i.Osd1[:]), binary.LittleEndian, masix1)
	return masix1
}

func (i *Inode) Linux2() *linux2 {
	linux2 := &linux2{}
	binary.Read(bytes.NewBuffer(i.Osd2[:]), binary.LittleEndian, linux2)
	return linux2
}

func (i *Inode) Hurd2() *hurd2 {
	hurd2 := &hurd2{}
	binary.Read(bytes.NewBuffer(i.Osd2[:]), binary.LittleEndian, hurd2)
	return hurd2
}

func (i *Inode) Masix2() *masix2 {
	masix2 := &masix2{}
	binary.Read(bytes.NewBuffer(i.Osd2[:]), binary.LittleEndian, masix2)
	return masix2
}

func (d *Device) NewInode(inodeNo uint32) (*Inode, error) {
	if inodeNo < 1 || inodeNo > d.InodesCount {
		return nil, errors.New(fmt.Sprintf("Inode %d out of bounds", inodeNo))
	}

	groupIndex := (inodeNo - 1) / d.InodesPerGroup
	group, err := d.NewGroupDescriptor(groupIndex)
	if err != nil {
		return nil, err
	}

	inodeIndex := (inodeNo - 1) % d.InodesPerGroup
	if inodeIndex >= d.InodesPerGroup {
		return nil, errors.New(fmt.Sprintf("Inode table index %d out of bounds", inodeIndex))
	}

	if _, err := d.seek(d.inodeOffset(group.InodeTable, inodeIndex)); err != nil {
		return nil, err
	}

	inode := &Inode{}
	if err := binary.Read(d.file, binary.LittleEndian, inode); err != nil {
		return nil, err
	}

	return inode, nil
}

func (d *Device) InodeFromPath(p string) (uint32, error) {
	if len(p) == 0 {
		return EXT2_ROOT_INO, nil
	}

	dirNames := strings.Split(path.Dir(p), "/")

	if strings.HasPrefix(p, "/") {
		dirNames = dirNames[1:]
	}

	if strings.HasSuffix(p, "/") {
		dirNames = dirNames[:len(dirNames)-1]
	}
	name := path.Base(p)

	currentInodeNo := uint32(EXT2_ROOT_INO)
	for _, dirName := range dirNames {
		dir, err := d.NewDirEntries(currentInodeNo)
		if err != nil {
			return EXT2_NULL_INO, err
		}

		if dirInodeNo := dir.Scan(dirName); dirInodeNo != EXT2_NULL_INO {
			currentInodeNo = dirInodeNo

			inode, err := d.NewInode(currentInodeNo)
			if err != nil {
				return EXT2_NULL_INO, err
			}

			if !inode.IsDir() {
				return EXT2_NULL_INO, nil
			}
		} else {
			return EXT2_NULL_INO, nil
		}
	}

	dir, err := d.NewDirEntries(currentInodeNo)
	if err != nil {
		return EXT2_NULL_INO, err
	}

	return dir.Scan(name), nil
}

func (d *Device) CreateDirInode(parent, inodeNo uint32) (*Inode, error) {
	groupNo := (inodeNo - 1) / d.InodesPerGroup

	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return nil, err
	}

	inode := &Inode{
		Mode:       S_IFDIR,
		UID:        uint16(os.Getuid()),
		GID:        uint16(os.Getgid()),
		Size:       1024,
		LinksCount: 1,
		Blocks:     2,
	}

	block, err := d.AllocBlock(groupNo)
	inode.Block[0] = block

	//Write Inode
	if _, err := d.seek(d.inodeOffset(group.InodeTable, (inodeNo-1)%d.InodesPerGroup)); err != nil {
		return nil, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, inode); err != nil {
		return nil, err
	}

	encode := func(entry *DirEntry) []byte {
		data := make([]byte, entry.RecLen)
		binary.LittleEndian.PutUint32(data[:4], entry.Inode)
		binary.LittleEndian.PutUint16(data[4:6], entry.RecLen)
		data[6] = byte(entry.NameLen)
		data[7] = byte(entry.FileType)
		copy(data[8:8+entry.NameLen], entry.Name[:entry.NameLen])

		return data
	}

	//Init directories
	inodeEntry := &DirEntry{
		Inode:    inodeNo,
		RecLen:   12,
		NameLen:  1,
		FileType: 2,
	}

	copy(inodeEntry.Name[:inodeEntry.NameLen], []byte("."))

	if _, err = d.file.WriteAt(encode(inodeEntry), d.blockOffset(block)); err != nil {
		return nil, err
	}

	if err := d.file.Sync(); err != nil {
		return nil, err
	}

	parentEntry := &DirEntry{
		Inode:    parent,
		RecLen:   uint16(d.BlockSize - uint32(inodeEntry.RecLen)),
		NameLen:  2,
		FileType: 2,
	}

	copy(parentEntry.Name[:parentEntry.NameLen], []byte(".."))

	if _, err = d.file.WriteAt(encode(parentEntry), d.blockOffset(block)+int64(inodeEntry.RecLen)); err != nil {
		return nil, err
	}

	return inode, d.file.Sync()
}

func (d *Device) CreateFileInode(inodeNo uint32) (*Inode, error) {
	groupNo := (inodeNo - 1) / d.InodesPerGroup

	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return nil, err
	}

	inode := &Inode{
		Mode:       S_IFREG,
		UID:        uint16(os.Getuid()),
		GID:        uint16(os.Getgid()),
		Size:       0,
		LinksCount: 1,
		Blocks:     0,
	}

	block, err := d.AllocBlock(groupNo)
	inode.Block[0] = block

	if _, err := d.seek(d.inodeOffset(group.InodeTable, (inodeNo-1)%d.InodesPerGroup)); err != nil {
		return nil, err
	}

	if err := binary.Write(d.file, binary.LittleEndian, inode); err != nil {
		return nil, err
	}

	return inode, d.file.Sync()
}

func (d *Device) UpdateInode(inodeNo uint32, value interface{}, offset int64) error {
	groupNo := (inodeNo - 1) / d.InodesPerGroup
	inodeIdx := (inodeNo - 1) % d.InodesPerGroup

	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return err
	}

	if _, err := d.seek(d.inodeOffset(group.InodeTable, inodeIdx) + offset); err != nil {
		return err
	}

	if err := binary.Write(d.file, binary.LittleEndian, value); err != nil {
		return err
	}

	return nil
}

func (i *Inode) IsReg() bool {
	return (i.Mode & S_IFREG) != 0
}

func (i *Inode) IsDir() bool {
	return (i.Mode & S_IFDIR) != 0
}

func (i *Inode) IsChr() bool {
	return (i.Mode & S_IFCHR) != 0
}

func (i *Inode) IsBlk() bool {
	return (i.Mode & S_IFBLK) != 0
}

func (i *Inode) IsFIFO() bool {
	return (i.Mode & S_IFIFO) != 0
}

func (i *Inode) IsLnk() bool {
	return (i.Mode & S_IFLNK) != 0
}

func (i *Inode) IsSock() bool {
	return (i.Mode & S_IFSOCK) != 0
}

//TODO: Add more cases
func (i *Inode) FileMode() os.FileMode {
	if i.IsDir() {
		return os.ModeDir | os.FileMode(0777)
	} else if i.IsReg() {
		return os.FileMode(0777)
	}
	return os.FileMode(0)
}
