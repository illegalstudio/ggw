//go:build linux

package cow

import (
	"fmt"
	"syscall"
)

// fsMagicNames maps the filesystem magic numbers a developer machine is likely
// to hit onto readable names, so a failed clone can say "ext4" rather than
// "0xef53". Unknown values fall back to the raw magic.
var fsMagicNames = map[int64]string{
	0x9123683e: "btrfs",
	0x58465342: "XFS",
	0xca451a4e: "bcachefs",
	0x2fc12fc1: "ZFS",
	0xef53:     "ext2/3/4",
	0xf2f52010: "F2FS",
	0x01021994: "tmpfs",
	0x858458f6: "ramfs",
	0x794c7630: "overlayfs",
	0x65735546: "FUSE",
	0x6969:     "NFS",
	0xff534d42: "CIFS/SMB",
	0x4d44:     "FAT",
	0x2011bab0: "exFAT",
	0x5346544e: "NTFS",
	0x73717368: "squashfs",
}

func deviceID(path string) (uint64, bool) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, false
	}
	return uint64(st.Dev), true
}

func filesystemName(path string) string {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return ""
	}
	if name, ok := fsMagicNames[int64(st.Type)]; ok {
		return name
	}
	return fmt.Sprintf("magic 0x%x", uint64(st.Type))
}
