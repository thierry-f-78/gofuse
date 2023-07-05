package main

import "io"
import "os"
import "os/user"
import "path/filepath"
import "strings"

func path_abs(path string)(string, error) {
	var err error
	var usr *user.User

	// Special case with ~
	if strings.HasPrefix(path, "~" + string(filepath.Separator)) {
		usr, err = user.Current()
		if err != nil {
			return "", err
		}
		path = usr.HomeDir + path[1:]
	}

	return filepath.Abs(path)
}

func is_empty_dir(name string) (bool) {
	var f *os.File
	var err error

	f, err = os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true
	}
	return false
}

