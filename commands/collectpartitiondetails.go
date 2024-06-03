package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"transformer/util"
)

type PartitionContentDetails struct {
	Device               string
	GrubCfgFound         bool
	GrubCfgPath          string
	GrubCfgContents      string
	EPICApplicationFound bool
	EPICApplicationPath  string
	FstabFound           bool
	FstabContents        string
	OSReleaseFound       bool
	OSRelease            OSRelease
}

type ErrThingFound struct {
	Path string
}

func (e *ErrThingFound) Error() string {
	return fmt.Sprintf("file found at %s", e.Path)
}

func IsThingFoundErr(err error) (errFileFound *ErrThingFound, ok bool) {
	ok = errors.As(err, &errFileFound)
	return errFileFound, ok
}

type ErrFileNotFound string

func (e ErrFileNotFound) Error() string {
	return string(e)
}

func IsErrFileNotFound(err error) bool {
	var e ErrFileNotFound
	return errors.As(err, &e)
}

func fileNotFoundErr(root, file string) error {
	return ErrFileNotFound(fmt.Sprintf("%s not found starting at %s", file, root))
}

type ErrDirNotFound string

func (e ErrDirNotFound) Error() string {
	return string(e)
}

func IsErrDirNotFound(err error) bool {
	var e ErrDirNotFound
	return errors.As(err, &e)
}

func dirNotFoundErr(root, file string) error {
	return ErrDirNotFound(fmt.Sprintf("%s not found starting at %s", file, root))
}

func (c *Commands) lookForFile(root, file string) (path string, err error) {
	e, _, d, err := util.WhatsThere(root)
	if err != nil || !e || !d {
		return "", fileNotFoundErr(root, file)
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if info.Name() == file {
			c.Debugf("found %s at %q\n", file, path)
			return &ErrThingFound{
				Path: path,
			}
		}

		return nil
	})
	if err != nil {
		if ffe, ok := IsThingFoundErr(err); ok {
			return ffe.Path, nil
		}
		return "", err
	}
	return "", fileNotFoundErr(root, file)
}

func (c *Commands) lookForDir(root, file string) (path string, err error) {
	e, _, d, err := util.WhatsThere(root)
	if err != nil || !e || !d {
		return "", dirNotFoundErr(root, file)
	}
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == file {
				c.Debugf("found %s at %q\n", file, path)
				return &ErrThingFound{
					Path: path,
				}
			}
		}

		return nil
	})
	if err != nil {
		if ffe, ok := IsThingFoundErr(err); ok {
			return ffe.Path, nil
		}
		return "", err
	}
	return "", dirNotFoundErr(root, file)
}

func (c *Commands) CollectPartitionDetails(device string) (result PartitionContentDetails, err error) {
	return WithGroupInScopeReturningErr(c, "CollectPartitionDetails", func(c *Commands) (result PartitionContentDetails, err error) {
		details, err := WithTempMount(c, device, "/mnt", func(c *Commands, mount MountDetails) (result PartitionContentDetails, err error) {
			result.Device = device
			grubRoot := filepath.Join(mount.MountPath, "grub")
			fstabPath := filepath.Join(mount.MountPath, "etc", "fstab")
			epicAppPath := filepath.Join(mount.MountPath, "home", "epicsystem")
			osReleasePaths := make([]string, len(DefaultOSReleaseSources))
			for i, source := range DefaultOSReleaseSources {
				osReleasePaths[i] = filepath.Join(mount.MountPath, source)
			}
			grubPath, err := c.lookForFile(grubRoot, "grub.cfg")
			if err == nil {
				result.GrubCfgFound = true
				bts, err := os.ReadFile(grubPath)
				if err != nil {
					return result, fmt.Errorf("error reading file at %s: %w", grubPath, err)
				}
				result.GrubCfgPath = grubPath
				result.GrubCfgContents = string(bts)
			}
			e, f, _, err := util.WhatsThere(fstabPath)
			if err == nil && e && f {
				result.FstabFound = true
				bts, err := os.ReadFile(fstabPath)
				if err != nil {
					return result, fmt.Errorf("error reading file at %s: %w", fstabPath, err)
				}
				result.FstabContents = string(bts)
			}
			e, _, d, err := util.WhatsThere(epicAppPath)
			if err == nil && e && d {
				result.EPICApplicationFound = true
				result.EPICApplicationPath = epicAppPath
			}
			osr, err := c.GetOSRelease(osReleasePaths)
			if err == nil {
				result.OSReleaseFound = true
				result.OSRelease = osr
			}
			return result, nil
		})
		if err != nil {
			return result, err
		}
		return details, nil
	})
}

func (c *Commands) CollectPartitionDetailsForPartitions(devices []string) (result []PartitionContentDetails, err error) {
	return WithGroupInScopeReturningErr(c, "CollectPartitionDetailsForPartitions", func(c *Commands) (result []PartitionContentDetails, err error) {
		for _, device := range devices {
			details, err := c.CollectPartitionDetails(device)
			if err != nil {
				return result, err
			}
			result = append(result, details)
		}
		return result, nil
	})
}
