package ext2fs

import (
	"encoding/binary"
	"errors"
	"io"
)

type DirEntry struct {
	Inode    uint32
	RecLen   uint16
	NameLen  uint8
	FileType uint8
	Name     [EXT2_NAME_LEN]byte
}

type DirEntries []*DirEntry

func (d DirEntries) Scan(name string) uint32 {
	for _, entry := range d {
		if entry.NameStr() == name {
			return entry.Inode
		}
	}
	return EXT2_NULL_INO
}

func (d DirEntries) Ls() (names []string) {
	for _, entry := range d {
		names = append(names, entry.NameStr())
	}

	return names
}

func (de *DirEntry) NameStr() string {
	return string(de.Name[:de.NameLen])
}

func (d *Device) NewDirEntries(inodeNo uint32) (dir DirEntries, err error) {
	inode, err := d.NewInode(inodeNo)
	if err != nil {
		return nil, err
	}

	if !inode.IsDir() {
		return nil, errors.New("Inode is not a directory")
	}

	blocks := inode.Size / d.BlockSize

	for i := uint32(0); i < blocks; i++ {
		block := make([]byte, d.BlockSize)

		_, err := d.ReadData(inode, block, int64(d.BlockSize*i))
		if err != nil && err != io.EOF {
			return nil, err
		}

		for len(block) > 8 {
			entry := &DirEntry{
				Inode:  binary.LittleEndian.Uint32(block[0:4]),
				RecLen: binary.LittleEndian.Uint16(block[4:6]),
			}

			if int(entry.RecLen) == 0 {
				//fmt.Println("Warn: bad directory entry encountered; record has zero length")
				break
			}

			if int(entry.RecLen) > len(block) {
				//fmt.Println("Warn: bad directory entry encountered; record is longer than remaining block")
				break
			}

			if int(block[6]+8) > len(block) {
				//fmt.Println("Warn: bad directory entry; name length longer than remaining block")
				break
			}

			if entry.Inode != EXT2_NULL_INO {
				entry.NameLen = uint8(block[6])
				entry.FileType = uint8(block[7])
				copy(entry.Name[:entry.NameLen], block[8:8+int16(entry.NameLen)])
				dir = append(dir, entry)
			}
			block = block[entry.RecLen:]
		}
	}

	return dir, nil
}

//Finds an offset for a new entry
//Returns -1 if entry doesn't fit in existing blocks
func (d *Device) DirEntryOffset(inode *Inode) (off int64, lastLen uint32, err error) {
	if !inode.IsDir() {
		return 0, 0, errors.New("Inode is not a directory")
	}

	blocks := inode.Size / d.BlockSize
	block := make([]byte, d.BlockSize)
	off += int64((blocks - 1) * d.BlockSize)

	if _, err := d.ReadData(inode, block, off); err != nil && err != io.EOF {
		return 0, 0, err
	}

	for len(block) > 0 {
		entry := &DirEntry{
			Inode:   binary.LittleEndian.Uint32(block[0:4]),
			RecLen:  binary.LittleEndian.Uint16(block[4:6]),
			NameLen: uint8(block[6]),
		}

		block = block[entry.RecLen:]
		if len(block) > 0 {
			off += int64(entry.RecLen)
		} else {
			lastLen = uint32((((8 + entry.NameLen) + 3) &^ 0x03))
			off += int64(lastLen) //Bit manipulation to get next multiple of 4
		}
	}

	return off, lastLen, nil
}

func (d *Device) AppendDirEntry(inodeNo uint32, dirEntry *DirEntry) error {
	inode, err := d.NewInode(inodeNo)
	if err != nil {
		return err
	}

	//Write in parent inode
	off, lastLen, err := d.DirEntryOffset(inode)
	if err != nil {
		return err
	}

	lastOff := off - int64(lastLen)

	//Insert Entry
	data := make([]byte, dirEntry.RecLen)
	binary.LittleEndian.PutUint32(data[:4], dirEntry.Inode)
	binary.LittleEndian.PutUint16(data[4:6], uint16(int64(d.BlockSize)-(off%int64(d.BlockSize))))
	data[6] = byte(dirEntry.NameLen)
	data[7] = byte(dirEntry.FileType)
	copy(data[8:8+dirEntry.NameLen], dirEntry.Name[:dirEntry.NameLen])

	if _, err := d.WriteData(inode, data, off); err != nil {
		return err
	}

	//Update Last Entry
	data = make([]byte, 2)
	binary.LittleEndian.PutUint16(data, uint16(lastLen))

	if _, err := d.WriteData(inode, data, lastOff+4); err != nil {
		return err
	}

	//Update Group Descriptor
	groupNo := (inodeNo - 1) / d.InodesPerGroup
	group, err := d.NewGroupDescriptor(groupNo)
	if err != nil {
		return err
	}

	if _, err := d.seek(d.groupDescriptorOffset(groupNo) + BG_USED_DIRS_COUNT); err != nil {
		return err
	}

	if err := binary.Write(d.file, binary.LittleEndian, group.UsedDirsCount+1); err != nil {
		return err
	}

	return d.file.Sync()
}
