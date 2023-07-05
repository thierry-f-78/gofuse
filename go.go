package main

import "fmt"
import "io/fs"
import "os"
import "path/filepath"

// Search git repository in parents directory
func get_parent_go(target string, root string)(string, error) {
	var new_target string
	var stat fs.FileInfo
	var err error

	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}

	for {

		// go to parent
		new_target, err = filepath.Abs(target + "/..")
		if err != nil {
			return "", err
		}

		// check for go.mod
		stat, err = os.Stat(new_target + "/go.mod")
		if err == nil {
			if stat.Mode() & fs.ModeType == 0 {	
				return new_target, nil
			}
		}

		// we reach system root
		if new_target == target {
			return "", fmt.Errorf("go project not found")
		}

		// we reach root
		if new_target == root {
			return "", fmt.Errorf("go project not found")
		}

		// next
		target = new_target
	}
}

