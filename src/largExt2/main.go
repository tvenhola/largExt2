package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"largExt2/ext2fs"
	"os"
)

var verbose = false
var latin1 = false
var dirs = 0
var files = 0
var bytes int64 = 0

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 2 {
		help()
		return
	}
	source := args[len(args)-2]
	dest := args[len(args)-1]

	if len(args) > 2 {
		for i := 0; i < len(args)-2; i++ {
			switch args[i] {
			case "verbose":
				verbose = true
			case "latin1":
				latin1 = true
			default:
				help()
				return
			}
		}
	}

	device, err := ext2fs.NewDevice(source)
	if err != nil {
		fmt.Printf("Can't open %s: %s\n", source, err.Error())
		return
	}
	defer device.Close()
	superBlock, _ := device.NewSuperBlock()
	size := uint64(superBlock.BlocksCount) * uint64(device.BlockSize)
	free := uint64(superBlock.FreeBlocksCount) * uint64(device.BlockSize)
	report(fmt.Sprintf("Size %d\n", size))
	report(fmt.Sprintf("Used %d\n", size-free))
	report(fmt.Sprintf("Free %d\n\n", free))
	report(fmt.Sprintf("Block Size %d\n\n", device.BlockSize))
	err = dumpDirInode(device, dest, ext2fs.EXT2_ROOT_INO)
	fmt.Printf("Written %d files (total %d bytes) in %d directories\n", files, bytes, dirs)
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}
}

func help() {
	fmt.Println("Usage: largeExt2 [verbose] [latin1] source destination")
	fmt.Println("\nDumps all files from EXT2 image source (block device or file) to destination.")
	fmt.Println("Destination must be a directory and the process must have a read-write access to source.")
	fmt.Println("verbose parameter turns on file copy and directory logging.")
	fmt.Println("latin1 parameter converts source file names from latin1 to utf8.")
}

func dumpDir(device *ext2fs.Device, dir *ext2fs.DirEntry, path string) error {
	err := os.Mkdir(path, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}
	dirs++
	return dumpDirInode(device, path, dir.Inode)
}

func dumpDirInode(device *ext2fs.Device, target string, inode uint32) error {
	dirEntries, err := device.NewDirEntries(inode)
	if err != nil {
		return err
	}
	directories := make([]*ext2fs.DirEntry, 0)
	for _, entry := range dirEntries {
		switch entry.FileType {
		case 1:
			dumpFile(target, entry, device)
		case 2:
			directories = append(directories, entry)
		default:
			fmt.Printf("WARNING: Unhandled file type %d in file %s\n", entry.FileType, entry.NameStr())
		}
	}

	for i, dir := range directories {
		if dir.NameStr() == "." || dir.NameStr() == ".." {
			continue
		}
		dirName := fileName(dir)
		report(fmt.Sprintf("Entering subdirectory %s (%d/%d)\n", dirName, i-2, len(directories)-2))
		err := dumpDir(device, dir, target+"/"+dirName)
		if err != nil {
			return err
		}
	}
	return nil
}

func dumpFile(destDir string, entry *ext2fs.DirEntry, device *ext2fs.Device) error {
	name := fileName(entry)
	inode, err := device.NewInode(entry.Inode)
	report(fmt.Sprintf("Dump file %s to %s (%d bytes)\n", name, destDir, inode.Size))
	if err != nil {
		return err
	}
	file, err := os.Create(destDir + "/" + name)
	if err != nil {
		return err
	}
	writer := bufio.NewWriter(file)
	reader := ext2fs.NewInodeReader(device, inode)
	written, err := io.Copy(writer, reader)
	if err != nil {
		return err
	}
	report(fmt.Sprintf("Written %d bytes", written))
	err = writer.Flush()
	files++
	bytes += written
	return err
}

func fileName(entry *ext2fs.DirEntry) string {
	buf := entry.Name[:entry.NameLen]
	if !latin1 {
		return string(buf)
	}
	return toUtf8(buf)
}

func toUtf8(latinBuf []byte) string {
	buf := make([]rune, len(latinBuf))
	for i, b := range latinBuf {
		buf[i] = rune(b)
	}
	return string(buf)
}

func report(str string) {
	if !verbose {
		return
	}
	fmt.Println(str)
}
