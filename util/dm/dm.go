package dm

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

var (
	dirPrefix = flag.String("dir_prefix", "/tmp/zzz/dir_prefix", "file containing hyperparameters of this experiment")
)

type BigTable struct {
	f *os.File
}

func NewBigTable(owner string, xid, wid int) (*BigTable, error) {
	logFName := filepath.Join(*dirPrefix, fmt.Sprintf("%d", xid), fmt.Sprintf("%d", wid), "bt")
	f, err := os.Create(logFName)
	if err != nil {
		return nil, errors.Wrap(err, "os.Create")
	}

	bt := &BigTable{}
	bt.f = f
	return bt, nil
}

func (bt *BigTable) Write(step int, val map[string]string) error {
	val["step"] = fmt.Sprintf("%d", step)
	timestamp := time.Now().UnixNano()
	val["timestamp"] = fmt.Sprintf("%d", timestamp)

	b, err := json.Marshal(val)
	if err != nil {
		return errors.Wrap(err, "json.Marshal")
	}
	b = append(b, '\n')
	if _, err := bt.f.Write(b); err != nil {
		return errors.Wrap(err, "f.Write")
	}
	return nil
}

func GetConfig(xid, wid int, v interface{}) error {
	hyperFile := filepath.Join(*dirPrefix, fmt.Sprintf("%d", xid), "hyper")
	f, err := os.Open(hyperFile)
	if err != nil {
		return errors.Wrap(err, "os.Open")
	}
	defer f.Close()
	lines := make([]string, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrap(err, "scanner.Err")
	}

	widLine := []byte(lines[wid])
	if err := json.Unmarshal(widLine, v); err != nil {
		return errors.Wrap(err, "json.Unmarshal")
	}
	return nil
}
