# git-fuse

This is an command line application
which parses a [git](https://git-scm.com/) repository and creates a readonly filesystem with its content.

## Filesystem

The filesystem is created with [FUSE](https://www.kernel.org/doc/html/latest/filesystems/fuse.html) and
with the cross-platform [cgofuse](https://github.com/billziss-gh/cgofuse) library.

For supported FUSE implementations check the [cgofuse](https://github.com/billziss-gh/cgofuse) documentation.

## Usage

```
git-fuse /path/to/git/repository commitlike mountpoint [FUSE paramters]
```

Example:

```powershell
.\git-fuse C:\test master Z:
```

```bash
./git-fuse /home/user/test master /mnt/test
```
