package assets

import (
	"fmt"
	"io/fs"
)

func ListAll() {
	Walk("", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println("err:", err)
			return nil
		}
		fmt.Println(path, d.IsDir())
		return nil
	})
}
