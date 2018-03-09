package file

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
)

func ReadFile(ctx context.Context, name string) ([]byte, error) {
	b, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, errors.Wrap(err, "ioutil.ReadAll")
	}
	return b, nil
}

func WriteFile(ctx context.Context, name string, body []byte) error {
	f, err := os.Create(name)
	if err != nil {
		return errors.Wrap(err, "os.Create")
	}
	defer f.Close()
	if _, err := f.Write(body); err != nil {
		return errors.Wrap(err, "file.Write")
	}
	return nil
}

func DeleteAll(ctx context.Context, name string) error {
	if err := os.RemoveAll(name); err != nil {
		return errors.Wrap(err, "os.RemoveAll")
	}
	return nil
}

func MkdirAll(ctx context.Context, name string, perm *os.FileMode) error {
	if perm == nil {
		var mode os.FileMode = 0755
		perm = &mode
	}
	if err := os.MkdirAll(name, *perm); err != nil {
		errors.Wrap(err, "os.MkdirAll")
	}
	return nil
}
