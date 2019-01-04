package ext2fs

const (
	EXT2_SUPERBLOCK_SIZE    = 1024
	EXT2_GROUP_DESC_SIZE    = 32
	EXT2_DEFAULT_BLOCK_SIZE = 1024
	EXT2_DEFAULT_INODE_SIZE = 128
	EXT2_SUPER_MAGIC        = 0xEF53
	EXT2_GOOD_OLD_REV       = 0
	EXT2_DYNAMIC_REV        = 1
	BASE_OFFSET             = 1024
	S_FREE_BLOCKS_COUNT     = 12
	S_FREE_INODES_COUNT     = S_FREE_BLOCKS_COUNT + 4
	BG_FREE_BLOCKS_COUNT    = 12
	BG_FREE_INODES_COUNT    = BG_FREE_BLOCKS_COUNT + 2
	BG_USED_DIRS_COUNT      = BG_FREE_INODES_COUNT + 2
	I_SIZE                  = 4
	I_BLOCKS                = 28
	I_BLOCK                 = 40
	EXT2_NAME_LEN           = 255
	EXT2_NDIR_BLOCKS        = 12
	EXT2_IND_BLOCK          = EXT2_NDIR_BLOCKS
	EXT2_DIND_BLOCK         = EXT2_IND_BLOCK + 1
	EXT2_TIND_BLOCK         = EXT2_DIND_BLOCK + 1
	EXT2_N_BLOCKS           = EXT2_TIND_BLOCK + 1
	EXT2_NULL_BLOCK         = 0
	EXT2_ROOT_INO           = 2
	EXT2_NULL_INO           = 0
	S_IFMT                  = 0170000
	S_IFSOCK                = 0140000
	S_IFLNK                 = 0120000
	S_IFREG                 = 0100000
	S_IFBLK                 = 0060000
	S_IFDIR                 = 0040000
	S_IFCHR                 = 0020000
	S_IFIFO                 = 0010000
)
