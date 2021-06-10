package main

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/go-git/go-git/v5/plumbing/object"
)

type Gitfs struct {
	fuse.FileSystemBase
	root      *object.Tree
	totalSize int64
	fileCount int
}

func getRelativePath(path string) string {
	if path == "" {
		return ""
	}
	return path[1:]
}

func (this *Gitfs) Open(path string, flags int) (errc int, fh uint64) {
	p := getRelativePath(path)
	_, err := this.root.File(p)
	if err != nil {
		return -fuse.ENOENT, ^uint64(0)
	}
	return 0, 0
}

func (this *Gitfs) Getattr(path string, stat *fuse.Stat_t, fh uint64) (errc int) {
	p := getRelativePath(path)
	if p == "" {
		stat.Mode = fuse.S_IFDIR | 0o00000755
		return 0
	}
	_, err := this.root.Tree(p)
	if err == nil {
		stat.Mode = fuse.S_IFDIR | 0o00000755
		return 0
	}
	file, err := this.root.File(p)
	if err != nil {
		return -fuse.ENOENT
	}
	stat.Size = file.Size
	stat.Mode = uint32(file.Mode)
	return 0
}

func (this *Gitfs) Read(path string, buff []byte, ofst int64, fh uint64) (n int) {
	p := getRelativePath(path)
	file, err := this.root.File(p)
	if err != nil {
		return -fuse.ENOENT
	}
	reader, err := file.Blob.Reader()
	if err != nil {
		return -fuse.ENOENT
	}
	io.CopyN(ioutil.Discard, reader, ofst)
	len, err := reader.Read(buff)
	if err != nil && err != io.EOF {
		return -fuse.ENOENT
	}
	return len
}

func (this *Gitfs) Release(path string, fh uint64) (errc int) {
	return 0
}

func (this *Gitfs) Opendir(path string) (errc int, fh uint64) {
	return 0, 0
}

func (this *Gitfs) Readdir(path string, fill func(name string, stat *fuse.Stat_t, ofst int64) bool, ofst int64, fh uint64) (errc int) {
	p := getRelativePath(path)
	var tree *object.Tree
	if p == "" {
		tree = this.root
	} else {
		var err error
		tree, err = this.root.Tree(p)
		if err != nil {
			return -fuse.EINVAL
		}
	}
	fill(".", nil, 0)
	fill("..", nil, 0)
	for _, entry := range tree.Entries {
		fill(entry.Name, nil, 0)
	}
	return 0
}

func (this *Gitfs) Releasedir(path string, fh uint64) (errc int) {
	return 0
}

func (this *Gitfs) Listxattr(path string, fill func(name string) bool) (errc int) {
	return 0
}

func (this *Gitfs) Statfs(path string, stat *fuse.Statfs_t) (errc int) {
	blocksize := uint64(4096)
	*stat = fuse.Statfs_t{
		Bfree:  0,
		Bavail: 0,
		Bsize:  blocksize,
		Favail: 0,
		Ffree:  0,
		Frsize: blocksize,
		Files:  uint64(this.fileCount),
		Blocks: uint64(this.totalSize) / blocksize,
	}
	return 0
}

func humanReadable(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	n = n / 1024
	if n < 1024 {
		return fmt.Sprintf("%dkB", n)
	}
	n = n / 1024
	if n < 1024 {
		return fmt.Sprintf("%dMB", n)
	}
	n = n / 1024
	return fmt.Sprintf("%dGB", n)
}

func NewGitfs(root *object.Tree) *Gitfs {
	totalSize := int64(0)
	fileCount := 0
	root.Files().ForEach(func(file *object.File) error {
		totalSize += file.Size
		fileCount++
		return nil
	})
	logger.Printf("Enumerated %d files with total size of %s", fileCount, humanReadable(totalSize))
	return &Gitfs{
		root:      root,
		totalSize: totalSize,
		fileCount: fileCount,
	}
}
