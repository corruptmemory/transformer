package commands

import (
	"io"
	"os"
	"transformer/bootutils"
)

func (c *Commands) CopyBootImages(src, dest string) error {
	files, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, file := range files {
		if bootutils.IsBootFile(file.Name()) {
			srcPath := src + "/" + file.Name()
			destPath := dest + "/" + file.Name()
			c.Infof("Copying %s to %s", srcPath, destPath)
			srcFile, err := os.Open(srcPath)
			if err != nil {
				return err
			}
			defer srcFile.Close()
			destFile, err := os.Create(destPath)
			if err != nil {
				return err
			}
			defer destFile.Close()
			_, err = io.Copy(destFile, srcFile)
			if err != nil {
				return err
			}
			err = destFile.Sync()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
