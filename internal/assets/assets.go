package assets

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed all:assets all:audit all:lib all:modes all:projects packages.conf firewall.conf env.template
var FS embed.FS

func ReadFile(path string) ([]byte, error) {
	return FS.ReadFile(path)
}

func Exists(path string) bool {
	_, err := fs.Stat(FS, path)
	return err == nil
}

func ExtractTo(path, dest string) error {
	data, err := FS.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}

func Walk(root string, fn func(path string, d fs.DirEntry, err error) error) error {
	return fs.WalkDir(FS, root, fn)
}