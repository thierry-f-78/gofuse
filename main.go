package main

import "encoding/json"
import "fmt"
import "io"
import "io/fs"
import "io/ioutil"
import "os"

import "github.com/go-git/go-git/v5"
import "github.com/go-git/go-git/v5/plumbing/filemode"
import "github.com/go-git/go-git/v5/plumbing/object"

func usage() {
	fmt.Fprintf(os.Stderr, "gofuse <target-dir> <remote-git> <git-spec>\n")
	os.Exit(1)
}

func main() {
	var arg_target string
	var target string
	var git_dir string
	var git_wt *git.Worktree
	var git_local *git.Repository
	var git_remote *git.Repository
	var err error
	var git_target string
	var git_refspec string
	var tree *object.Tree
	var files []string
	var fh *os.File
	var jenc *json.Encoder
	var rel string
	var gomod []byte
	var git_id string

	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "gofuse require 3 arguments, got %d\n", len(os.Args) - 1)
		usage()
	}

	arg_target = os.Args[1]
	git_target = os.Args[2]
	git_refspec = os.Args[3]

	// Get absolute path of target. Target must not exists, or must be a directory
	target, err = path_abs(arg_target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error with target directory %q: %s\n", arg_target, err.Error())
		os.Exit(1)
	}

	// Search git directory
	git_local, git_dir, err = get_parent_git(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening parent git directory of %q: %s\n", target, err.Error())
		os.Exit(1)
	}

	// Open worktree
	git_wt, err = git_local.Worktree()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening worktree directory of %q: %s\n", target, err.Error())
		os.Exit(1)
	}

	// Git directory is clean
	err = git_is_clean(git_local)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking if git directory %q is clean: %s\n", target, err.Error())
		os.Exit(1)
	}

	// Check for go.mod file
	_, err = get_parent_go(target, git_dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching go.mod in %q: %s\n", target, err.Error())
		os.Exit(1)
	}

	// Open remote git directory
	fmt.Printf("git clone %s\n", git_target)
	git_remote, err = git_open_remote(git_target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening remote git directory %q: %s\n", git_target, err.Error())
		os.Exit(1)
	}

	// Search refspec in remote dir
	fmt.Printf("git checkout %s\n", git_refspec)
	tree, git_id, err = git_object(git_remote, git_refspec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching refspec %q in %q: %s\n", git_refspec, git_target, err.Error())
		os.Exit(1)
	}

	// make target directory
	os.MkdirAll(target, 0755)

	// dump tree except go.mod, go.sum, return go.mod content
	rel = target[len(git_dir) + 1:]
	gomod, err = dump_tree(git_local, git_remote, target, rel, tree, &files)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dumping files: %s\n", err.Error())
		os.Exit(1)
	}

	// Remove old files
	remove_tree(git_wt, target, rel, files)

	// dump state file
	os.Remove(target + "/gofuse.json")
	fh, err = os.Create(target + "/gofuse.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file %q: %s\n", target + "/gofuse.json", err.Error())
		os.Exit(1)
	}
	jenc = json.NewEncoder(fh)
	err = jenc.Encode(&State{
		Version: 1,
		Repo: git_target,
		Refspec: git_refspec,
		Id: git_id,
		Files: files,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error write state file %q: %s\n", target + "/gofuse.json", err.Error())
		os.Exit(1)
	}
	fh.Close()

	// Add to the versionning
	fmt.Printf("git add %s\n", rel + "/gofuse.json")
	err = git_wt.AddWithOptions(&git.AddOptions{Path: rel + "/gofuse.json"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running git add %q: %s", rel + "/gofuse.json", err.Error())
		os.Exit(1)
	}

	if gomod == nil { print("") }
}

func dump_tree(git_local *git.Repository, git_remote *git.Repository, target string, rel string, tree *object.Tree, files *[]string)([]byte, error) {
	var git_wt *git.Worktree
	var entry object.TreeEntry
	var subtree *object.Tree
	var err error
	var perm os.FileMode
	var blob *object.Blob
	var reader io.ReadCloser
	var fh *os.File
	var data []byte
	var l int
	var new_gomod []byte
	var gomod []byte

	git_wt, err = git_local.Worktree()
	if err != nil {
		return nil, err
	}

	for _, entry = range tree.Entries {

		// Ignore submodule
		if entry.Mode == filemode.Submodule {
			continue
		}

		// Ignore empty
		if entry.Mode == filemode.Empty {
			continue
		}

		// Ignore go.sum
		if entry.Name == "go.sum" {
			continue
		}

		// Execute recursive call for directories
		if entry.Mode == filemode.Dir {
			subtree, err = git_remote.TreeObject(entry.Hash)
			if err != nil {
				return nil, fmt.Errorf("lookup for tree: %s", err.Error())
			}
			os.Mkdir(target + "/" + entry.Name, 0755)
			new_gomod, err = dump_tree(git_local, git_remote, target + "/" + entry.Name, rel + "/" + entry.Name, subtree, files)
			if err != nil {
				return nil, fmt.Errorf("%q: %s", rel, err.Error())
			}
			if new_gomod != nil {
				gomod = new_gomod
			}
			continue
		}

		// Get the blob object from the repository
		blob, err = git_remote.BlobObject(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("lookup for file %q: %s", entry.Name, err.Error())
		}

		// Ask for a Reader
		reader, err = blob.Reader()
		if err != nil {
			return nil, fmt.Errorf("can't open remote file %q: %s", rel + "/" + entry.Name, err.Error())
		}

		// process go.mod
		if entry.Name == "go.mod" {
			gomod, err = ioutil.ReadAll(reader)
			if err != nil {
				return nil, fmt.Errorf("can't read remote link %q: %s", rel + "/" + entry.Name, err.Error())
			}
			reader.Close()
			continue
		}

		// Remove target file
		os.Remove(target + "/" + entry.Name)

		// Open file for write
		if entry.Mode == filemode.Regular ||
		   entry.Mode == filemode.Deprecated ||
		   entry.Mode == filemode.Executable {

			// Check file mode
			switch entry.Mode {
			case filemode.Regular,
			     filemode.Deprecated:
				perm = 0644
			case filemode.Executable:
				perm = 0755
			}

			// Open target file
			fh, err = os.OpenFile(target + "/" + entry.Name, os.O_WRONLY|os.O_CREATE, perm)
			if err != nil {
				return nil, fmt.Errorf("can't open target file %q: %s", target + "/" + entry.Name, err.Error())
			}

			// Copy by bloc and compute sha1
			data = make([]byte, 4096)
			for {
				l, err = reader.Read(data)
				if l > 0 {
					_, err = fh.Write(data[:l])
					if err != nil {
						return nil, fmt.Errorf("can't write target file %q: %s", target + "/" + entry.Name, err.Error())
					}
				}
				if err != nil {
					if err == io.EOF {	
						break
					}
					return nil, fmt.Errorf("can't read remote file %q: %s", rel + "/" + entry.Name, err.Error())
				}
			}
			fh.Close()
			reader.Close()
		}

		if entry.Mode == filemode.Symlink {

			// Readlink at once
			data, err = ioutil.ReadAll(reader)
			if err != nil {
				return nil, fmt.Errorf("can't read remote link %q: %s", rel + "/" + entry.Name, err.Error())
			}
			reader.Close()

			// Create link
			err = os.Symlink(string(data), target + "/" + entry.Name)
			if err != nil {
				return nil, fmt.Errorf("can't write target link %q: %s", target + "/" + entry.Name, err.Error())
			}
		}

		// Append file to the list of files
		*files = append(*files, rel + "/" + entry.Name)

		// Perform git add
		fmt.Printf("git add %s\n", rel + "/" + entry.Name)
		err = git_wt.AddWithOptions(&git.AddOptions{Path: rel + "/" + entry.Name})
		if err != nil {
			return nil, fmt.Errorf("Error running git add %q: %s", rel + "/" + entry.Name, err.Error())
		}
 	}

 	return gomod, nil
}

func remove_tree(git_wt *git.Worktree, target string, rel string, files []string) {
	var entries []fs.DirEntry
	var entry fs.DirEntry
	var err error
	var fn string
	var s string
	var found bool
	var stat fs.FileInfo

	entries, err = os.ReadDir(target)
	if err != nil {
		return
	}

	for _, entry = range entries {
		stat, err = os.Stat(target + "/" + entry.Name())
		if err != nil {
			continue
		}
		if entry.Name() == "gofuse.json" {
			continue
		}
		if stat.IsDir() {
			remove_tree(git_wt, target + "/" + entry.Name(), rel + "/" + entry.Name(), files)
			if is_empty_dir(target + "/" + entry.Name()) {
				os.RemoveAll(target + "/" + entry.Name())
			}
		} else {
			fn = rel + "/" + entry.Name()
			found = false
			for _, s = range files {
				if s == fn {
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("git rm %s\n", fn)
				git_wt.Remove(fn)
				os.Remove(target + "/" + entry.Name())
			}
		}
	}
}
