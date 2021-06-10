package main

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func listenCommits(basePath string, commitRef string, ch chan<- *object.Tree) error {
	logger.Printf("Opening git repository at %s", basePath)
	repository, err := git.PlainOpen(basePath)
	if err != nil {
		return errors.New(fmt.Sprintf("Failed to open repository at %s: %v", basePath, err))
	}
	logger.Printf("Git repository is opened at %s", basePath)

	err = handleBranch(repository, commitRef, ch, filepath.Join(basePath, ".git"))
	if err == nil {
		return nil
	}

	err = handleObject(repository, commitRef, ch)
	if err == nil {
		return nil
	}

	err = handleTag(repository, commitRef, ch)
	if err == nil {
		return nil
	}

	return errors.New("Cannot handle commitref")
}

func handleBranch(repository *git.Repository, commitRef string, ch chan<- *object.Tree, gitPath string) error {
	branch, err := repository.Branch(commitRef)
	if err != nil {
		return err
	}
	ref, err := repository.Reference(branch.Merge, true)
	if err != nil {
		return err
	}
	tree, err := getTree(repository, ref.Hash())
	if err != nil {
		return err
	}
	logger.Printf("Branch %s is resolved to %s", commitRef, tree.Hash.String())

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Printf("Failed to initialize file watcher, branch will not be followed")
	} else {
		file := filepath.Join(gitPath, string(branch.Merge))
		dir := filepath.Dir(file)
		err = watcher.Add(dir)
		if err != nil {
			logger.Printf("Failed to initialize file watcher for %s, branch will not be followed", file)
			watcher.Close()
		} else {
			logger.Printf("Watching directory %s", dir)
			go func() {
				defer watcher.Close()
				for {
					select {
					case event, ok := <-watcher.Events:
						logger.Print(event.Name)
						if !ok || event.Name != file {
							continue
						}
						if event.Op&fsnotify.Create == fsnotify.Create {
							logger.Printf("Start following branch %s", branch.Merge)
							err = handleFileChange(repository, branch.Merge, ch)
							if err != nil {
								logger.Printf("Failed to follow branch %s: %s", branch.Merge.String(), err.Error())
							}
						}
						if event.Op&fsnotify.Write == fsnotify.Write {
							err = handleFileChange(repository, branch.Merge, ch)
							if err != nil {
								logger.Printf("Failed to follow branch %s: %s", branch.Merge.String(), err.Error())
							}
						}
						if event.Op&fsnotify.Remove == fsnotify.Remove {
							logger.Printf("Stop following branch %s", branch.Merge)
						}
					case err, ok := <-watcher.Errors:
						if !ok {
							break
						}
						logger.Printf("File watcher error: %s", err.Error())
					}
				}
			}()
		}
	}
	ch <- tree
	return nil
}

func handleFileChange(repository *git.Repository, merge plumbing.ReferenceName, ch chan<- *object.Tree) error {
	ref, err := repository.Reference(merge, true)
	if err != nil {
		return err
	}
	tree, err := getTree(repository, ref.Hash())
	if err != nil {
		return err
	}
	logger.Printf("Branch %s is updated to %s", merge.String(), tree.Hash.String())
	ch <- tree
	return nil
}

func handleObject(repository *git.Repository, commitRef string, ch chan<- *object.Tree) error {
	if !plumbing.IsHash(commitRef) {
		return errors.New("This is not a hash")
	}
	tree, err := getTree(repository, plumbing.NewHash(commitRef))
	if err != nil {
		return err
	}
	logger.Printf("Hash %s is recognised", tree.Hash.String())
	ch <- tree
	return nil
}

func handleTag(repository *git.Repository, commitRef string, ch chan<- *object.Tree) error {
	tag, err := repository.Tag(commitRef)
	if err != nil {
		return err
	}
	tree, err := getTree(repository, tag.Hash())
	if err != nil {
		return err
	}
	logger.Printf("Tag %s is resolved to %s", commitRef, tree.Hash.String())
	ch <- tree
	return nil
}

func getTree(repository *git.Repository, hash plumbing.Hash) (*object.Tree, error) {
	commit, err := repository.CommitObject(hash)
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	return tree, nil
}
