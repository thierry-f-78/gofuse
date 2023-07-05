package main

import "fmt"
import "io/fs"
import "os"
import "path/filepath"

import "github.com/go-git/go-git/v5"
import "github.com/go-git/go-git/v5/plumbing"
import "github.com/go-git/go-git/v5/plumbing/object"
import "github.com/go-git/go-git/v5/storage/memory"

// Search git repository in parents directory
func get_parent_git(target string)(*git.Repository, string, error) {
	var new_target string
	var stat fs.FileInfo
	var err error
	var git_dit *git.Repository

	for {

		// go to parent
		new_target, err = filepath.Abs(target + "/..")
		if err != nil {
			return nil, "", err
		}

		// we reach root
		if new_target == target {
			return nil, "", fmt.Errorf("repository not found")
		}
		target = new_target

		// check for git
		stat, err = os.Stat(target + "/.git")
		if err != nil {
			continue
		}

		// stop if we reach git directory
		if stat.IsDir() {
			break
		}
	}

	// Open git repository
	git_dit, err = git.PlainOpen(target)
	if err != nil {
		return nil, "", err
	}
	return git_dit, target, nil
}

func git_is_clean(git_dir *git.Repository)(error) {
	var git_wt *git.Worktree 
	var st git.Status
	var err error
	var fs *git.FileStatus

	git_wt, err = git_dir.Worktree()
	if err != nil {
		return err
	}

	st, err = git_wt.Status()
	if err != nil {
		return err
	}

	for _, fs = range st {
		if (fs.Staging != git.Unmodified && fs.Staging != git.Untracked) &&
		   (fs.Worktree != git.Unmodified && fs.Worktree != git.Untracked) {
			return fmt.Errorf("git repository not clean")
		}
	}

	return nil
}

func git_open_remote(remote string)(*git.Repository, error) {
	return git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL: remote,
	})
}

func git_object(repo *git.Repository, name string)(*object.Tree, string, error) {
	var commit *object.Commit
	var h *plumbing.Hash
	var tree *object.Tree
	var err error

	h, err = repo.ResolveRevision(plumbing.Revision(name))
	if err != nil {
		return nil, "", err
	}

	commit, err = repo.CommitObject(*h)
	if err != nil {
		return nil, "", err
	}

	tree, err = repo.TreeObject(commit.TreeHash)
	if err != nil {
		return nil, "", err
	}

	return tree, commit.Hash.String(), nil
}
