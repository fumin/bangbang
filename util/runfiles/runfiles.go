package runfiles

import (
	"flag"
	"os"
	"path/filepath"
)

var (
	defaultPrefix = filepath.Join(os.Getenv("GOPATH"), "src")
	prefix        = flag.String("runfiles", defaultPrefix, "folder containing our run files")
)

func Path(inpath string) string {
	return filepath.Join(*prefix, inpath)
}
