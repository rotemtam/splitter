package splitter

import (
	"github.com/rogpeppe/go-internal/testscript"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"splitter": Run,
	}))
}

func TestScript(t *testing.T) {
	testscript.Run(t, testscript.Params{
		Dir:  "testdata",
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){},
	})
}
