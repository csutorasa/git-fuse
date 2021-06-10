package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/billziss-gh/cgofuse/fuse"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const appName = "git-fuse"

var logger *log.Logger = log.Default()

func main() {
	basePath, commitRef, mountPoint, fuseOpts := parseArgs()

	ch := make(chan *object.Tree, 10)
	err := listenCommits(basePath, commitRef, ch)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	interruptChannel := make(chan int, 3)
	interruptHandler(interruptChannel)

	var host *fuse.FileSystemHost
	var hash string
	for {
		select {
		case tree := <-ch:
			if tree.Hash.String() == hash {
				break
			}
			fs := NewGitfs(tree)
			hash = tree.Hash.String()
			if host != nil {
				logger.Print("Stopping filesystem")
				if host.Unmount() {
					for host.Unmount() {

					}
				}
				logger.Print("Filesystem stopped")
			}
			host = fuse.NewFileSystemHost(fs)
			// TODO host.SetCapReaddirPlus(true)
			go func() {
				logger.Printf("Starting filesystem on %s with args: %s", mountPoint, strings.Join(fuseOpts, " "))
				host.Mount(mountPoint, fuseOpts)
			}()
		case <-interruptChannel:
			if host != nil {
				logger.Print("Stopping filesystem")
				host.Unmount()
				logger.Print("Filesystem stopped")
			}
			break
		}
	}
}

func printUsage() {
	fmt.Printf("Usage: %s /path/to/repository commitlike mountpoint [parameters to FUSE implementation]", appName)
	fmt.Println()
}

func parseArgs() (string, string, string, []string) {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Git repository path is missing")
		printUsage()
		os.Exit(1)
	}
	basePath := flag.Arg(0)
	if basePath == "-h" || basePath == "--help" {
		printUsage()
		os.Exit(0)
	}
	if flag.NArg() < 2 {
		fmt.Println("Commitlike is missing")
		printUsage()
		os.Exit(1)
	}
	commitRef := flag.Arg(1)
	if flag.NArg() < 3 {
		fmt.Println("Mount point is missing")
		printUsage()
		os.Exit(1)
	}
	mountPoint := flag.Arg(2)
	fuseOpts := flag.Args()[3:]
	return basePath, commitRef, mountPoint, fuseOpts
}

func interruptHandler(channel chan<- int) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT)
	closing := false
	go func() {
		for {
			switch <-c {
			case syscall.SIGINT:
				if !closing {
					closing = true
					logger.Printf("SIGINT received, graceful shutdown initiated")
					go func() {
						channel <- 1
					}()
				} else {
					logger.Printf("SIGINT received again, force shutdown initiated")
					os.Exit(0)
				}
				break
			case syscall.SIGTERM:
				logger.Printf("SIGTERM received, force shutdown initiated")
				os.Exit(0)
			case syscall.SIGABRT:
				logger.Printf("SIGABRT received, force shutdown initiated")
				os.Exit(0)
			}
		}
	}()
}
