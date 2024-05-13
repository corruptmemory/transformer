package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"transformer/util"
)

type GrubCfgDetails struct {
	Device   string
	Path     string
	Contents string
}

type ErrFileFound struct {
	Path string
}

func (e *ErrFileFound) Error() string {
	return fmt.Sprintf("file found at %s", e.Path)
}

func IsFileFoundErr(err error) (errFileFound *ErrFileFound, ok bool) {
	ok = errors.As(err, &errFileFound)
	return errFileFound, ok
}

var ErrGrubCfgNotFound = fmt.Errorf("grub.cfg not found")

func (c *Commands) FindGrubCfg(devices []string) (result []GrubCfgDetails, err error) {
	return WithGroupInScopeReturningErr(c, "FindGrubCfg", func(c *Commands) (result []GrubCfgDetails, err error) {
		for _, device := range devices {
			details, err := WithTempMount(c, device, "/mnt", func(c *Commands, mount MountDetails) (result GrubCfgDetails, err error) {
				e, _, d, err := util.WhatsThere(filepath.Join(mount.MountPath, "grub"))
				if err != nil || !e || !d {
					return result, ErrGrubCfgNotFound
				}
				err = filepath.Walk(filepath.Join(mount.MountPath, "grub"), func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}

					if info.IsDir() {
						return nil
					}

					if info.Name() == "grub.cfg" {
						c.Debugf("found grub.cfg at %q\n", path)
						return &ErrFileFound{
							Path: path,
						}
					}

					return nil
				})
				if err != nil {
					if ffe, ok := IsFileFoundErr(err); ok {
						bts, err := os.ReadFile(ffe.Path)
						if err != nil {
							return result, fmt.Errorf("error reading file at %s: %w", ffe.Path, err)
						}
						result.Device = device
						result.Path = ffe.Path
						result.Contents = string(bts)
						return result, nil
					}
					return result, err
				}
				return result, ErrGrubCfgNotFound
			})
			if err != nil && errors.Is(err, ErrGrubCfgNotFound) {
				continue
			} else if err != nil {
				return result, err
			}
			result = append(result, details)
		}
		return result, nil
	})
}
