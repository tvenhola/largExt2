package ext2fs

type InodeReader struct {
	Device  *Device
	Inode   *Inode
	CurrPos int64
	EOF     bool
}

func (r *InodeReader) Read(p []byte) (n int, err error) {
	n, err = r.Device.ReadData(r.Inode, p, r.CurrPos)
	if int64(n)+r.CurrPos > int64(r.Inode.Size) {
		n = int(int64(r.Inode.Size) - r.CurrPos)
	}
	r.CurrPos += int64(n)
	return
}

func NewInodeReader(device *Device, inode *Inode) (inodeReader *InodeReader) {
	inodeReader = new(InodeReader)
	inodeReader.Device = device
	inodeReader.Inode = inode
	return
}
