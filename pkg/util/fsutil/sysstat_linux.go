package fsutil

import (
	"os"
	"syscall"
)

func SysStatInfo(info os.FileInfo) *SysStat {
	raw, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	return &SysStat{
		Ok:    true,
		Uid:   raw.Uid,
		Gid:   raw.Gid,
		Atime: raw.Atim,
		Mtime: raw.Mtim,
		Ctime: raw.Ctim,
	}
}
