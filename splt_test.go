package splitter

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"github.com/rogpeppe/go-internal/testscript"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"splt": Run,
	}))
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir: "testdata",
		Setup: func(env *testscript.Env) error {
			home, err := homedir.Dir()
			if err != nil {
				return err
			}
			env.Setenv("HOME", home)
			return nil
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"cmpdir": func(ts *testscript.TestScript, neg bool, args []string) {
				if len(args) != 2 && !neg {
					ts.Fatalf("usage: cmpdir dir1 dir2")
				}
				// Get the temp dir from the test script env vars:
				dir1, dir2 := ts.MkAbs(args[0]), ts.MkAbs(args[1])
				if err := cmpDirs(dir1, dir2); err != nil {
					if neg {
						return
					}
					ts.Fatalf("compare failed: %v", err)
				}
				if neg {
					ts.Fatalf("compare succeeded, but expected failure")
				}
			},
		},
	})
}

// cmpDirs compares the contents of two directories.
func cmpDirs(dir1, dir2 string) error {
	// First compile the file lists and compare all files are equivalent.
	files1, err := sortedFiles(dir1)
	if err != nil {
		return err
	}
	files2, err := sortedFiles(dir2)
	if err != nil {
		return err
	}
	if len(files1) != len(files2) {
		return fmt.Errorf("different number of files: %d vs %d", len(files1), len(files2))
	}
	for _, fn := range files1 {
		f1, err := os.ReadFile(fn)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", fn, err)
		}
		relPath, err := filepath.Rel(dir1, fn)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %v", err)
		}
		f2, err := os.ReadFile(filepath.Join(dir2, relPath))
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", fn, err)
		}
		if string(f1) != string(f2) {
			return fmt.Errorf("file %s differs", fn)
		}
	}
	return nil
}

// sortedFiles returns a sorted slice of file paths for all files (including in subdirectories) of a directory
func sortedFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
