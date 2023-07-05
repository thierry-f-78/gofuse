This program aims to embed the code of a Go library into a program.

Why?
----

Sometimes, we have various tool libraries that we maintain in a specific internal
repository within our company. These libraries are not strategic but rather
helpers or specific protocols. We use these libraries in a larger program.

One day, we want to share or publish this program, but it would require sharing
all the internal libraries. However, this sharing work is substantial. It
involves moving the source repository, documenting it, and changing all the
pointers to these libraries in all the Go programs that use them.

This tool allows us to embed libraries directly into the code. This way, the
versioning doesn't change location, and the code of the dependency is integrated
into the main program as a snapshot.

How?
----

The source project must be a versioned Go project with Git, without any pending
data for commit; otherwise, gofuse will refuse to work. The project to import
must necessarily be a Git repository.

gofuse checks its environment:

- A valid Git versioning.

- No cached data in Git.

- The presence of a go.mod file in one of the ancestors of the current path, but
  inside the Git repository.

gofuse clones the project to merge it into a temporary directory. It processes
all the files related to the commit ID, tag, or specified branch. It writes and
adds the new files to the parent repository, then deletes the old files from the
parent repository. If it encounters a go.mod file, it analyzes it and sets aside
the dependencies to be imported. Once the files are processed, it attempts to
import the dependencies. Finally, it writes a state file that keeps an accurate
record of the performed action.

The import and update process goes as follows:

- `git` `status`
- `git` `clone` *dest* *tmp*
- `git` `archive` *tmp* *spec*
- `git` `add` *newfile*
- `git` `rm` *oldfile*
- create *statefile*
- `git` `add` *statefile*
- remote go.mod analysis
- `go` `get` *module*@*version*
- `git` `add` */go.mod*
- `git` `add` */go.sum*

The user can make a git commit if satisfied or a git checkout -f.

Usage
-----

To load or update a project:

   `gofuse <target-directory> <git-repository-path> <version>`
