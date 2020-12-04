// Command git-annex-remote-local is a simple file-based external special remote for git-annex. It
// is meant as a demonstration of the usage of the remote package; in practice, git-annex's native
// directory special remote should be used instead.
package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	remote "github.com/dzhu/git-annex-remote"
)

const rootConfigName = "root"

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

type fileRemote struct {
	root string
}

func (f *fileRemote) getPath(key string) string {
	return filepath.Join(f.root, key)
}

func (f *fileRemote) Init(a remote.Annex) error {
	root := a.GetConfig(rootConfigName)
	if root == "" {
		return errors.New("must provide root directory")
	}
	return os.MkdirAll(root, 0o700)
}

func (f *fileRemote) Prepare(a remote.Annex) error {
	f.root = a.GetConfig(rootConfigName)
	a.Infof("prepared with root %s", f.root)
	return nil
}

func (f *fileRemote) Store(a remote.Annex, key, file string) error {
	a.Infof("copying %s -> %s", file, f.getPath(key))
	return copyFile(file, f.getPath(key))
}

func (f *fileRemote) Retrieve(a remote.Annex, key, file string) error {
	a.Infof("copying %s -> %s", f.getPath(key), file)
	return copyFile(f.getPath(key), file)
}

func (f *fileRemote) Present(a remote.Annex, key string) (bool, error) {
	switch _, err := os.Stat(f.getPath(key)); {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}

func (f *fileRemote) Remove(a remote.Annex, key string) error {
	err := os.Remove(f.getPath(key))
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return err
}

func (f *fileRemote) Extensions(a remote.Annex, es []string) []string {
	return []string{"INFO", "ASYNC"}
}

func (f *fileRemote) ListConfigs(a remote.Annex) []remote.ConfigSetting {
	return []remote.ConfigSetting{
		{Name: "root", Description: "the root directory"},
	}
}

// Statically ensure that the remote correctly implements the desired optional interfaces.
var (
	_ remote.HasExtensions  = (*fileRemote)(nil)
	_ remote.HasListConfigs = (*fileRemote)(nil)
)

func main() {
	remote.Run(&fileRemote{})
}
