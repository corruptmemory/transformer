package commands

import (
	"fmt"
	"strings"

	"transformer/util"
)

type MountDetails struct {
	Device     string
	MountPath  string
	canUnmount bool
}

func (c *Commands) TryMountAt(device, mountPoint string) (result MountDetails, err error) {
	return WithGroupInScopeReturningErr(c, "TryMountAt", func(c *Commands) (result MountDetails, err error) {
		// See if the device is already mounted
		mountCmd := c.Cmd("mount")
		err = c.RunCmd(mountCmd)
		if err != nil {
			return result, err
		}
		mountOutput := mountCmd.GetStdoutAsString()
		mg, err := util.NewGrepFromStrings(device)
		if err != nil {
			return result, err
		}
		mounted := mg.MatchLinesInString(mountOutput)
		if len(mounted) > 0 {
			c.Debugf("device %s is already mounted", device)
			ml := mounted[0]
			mlParts := util.RemoveEmptySplits(strings.Split(ml, " on "), true)
			if len(mlParts) != 2 {
				return result, fmt.Errorf("unexpected mount line: %s", ml)
			}
			mountParts := util.RemoveEmptySplits(strings.Split(mlParts[1], " "), true)
			if len(mountParts) == 0 {
				return result, fmt.Errorf("unexpected mount line: %s", ml)
			}
			result.Device = device
			result.MountPath = mountParts[0]
			return result, nil
		}
		mountCmd = c.Cmd("mount", device, mountPoint)
		err = c.RunCmd(mountCmd)
		if err != nil {
			return result, err
		}
		result.Device = device
		result.MountPath = mountPoint
		result.canUnmount = true
		return result, nil
	})
}

func (c *Commands) TryUnmount(mount MountDetails, recursive bool) (err error) {
	return c.WithGroupInScopeErr("TryUnmount", func(c *Commands) error {
		if !mount.canUnmount {
			c.Debugf("cannot unmount %s", mount.MountPath)
			return nil
		}
		var args []string
		if recursive {
			args = append(args, "-R")
		}
		args = append(args, mount.MountPath)
		umountCmd := c.Cmd("umount", args...)
		err := c.RunCmd(umountCmd)
		if err != nil {
			return err
		}
		return nil
	})
}

func WithTempMount[T any](c *Commands, device, mountPoint string, body func(c *Commands, mount MountDetails) (T, error)) (result T, err error) {
	return WithGroupInScopeReturningErr(c, "WithTempMount", func(c *Commands) (result T, err error) {
		mount, err := c.TryMountAt(device, mountPoint)
		if err != nil {
			return result, err
		}
		defer func() {
			err := c.TryUnmount(mount, false)
			if err != nil {
				c.Warnf("error unmounting %s: %v", mount.MountPath, err)
			}
		}()
		return body(c, mount)
	})
}

func (c *Commands) WithTempMountErr(device, mountPoint string, body func(c *Commands, mount MountDetails) error) (err error) {
	return c.WithGroupInScopeErr("WithTempMountErr", func(c *Commands) (err error) {
		mount, err := c.TryMountAt(device, mountPoint)
		if err != nil {
			return err
		}
		defer func() {
			err := c.TryUnmount(mount, false)
			if err != nil {
				c.Warnf("error unmounting %s: %v", mount.MountPath, err)
			}
		}()
		return body(c, mount)
	})
}

func (c *Commands) WithChrootMountConfig(mountRoot, rootDevice, grubDevice, efiDevice string, body func(c *Commands, mount MountDetails) error) (err error) {
	return c.WithGroupInScopeErr("WithChrootMountConfig", func(c *Commands) error {
		err := c.WithTempMountErr(rootDevice, mountRoot, func(c *Commands, rootMount MountDetails) error {
			c.Infof("  > Mounted %s at %s", rootDevice, rootMount.MountPath)
			err := c.WithTempMountErr(grubDevice, mountRoot+"/boot", func(c *Commands, grubMount MountDetails) error {
				c.Infof("  > Mounted %s at %s", grubDevice, grubMount.MountPath)
				err := c.WithTempMountErr(efiDevice, mountRoot+"/boot/efi", func(c *Commands, efiMount MountDetails) error {
					c.Infof("  > Mounted %s at %s", efiMount, efiMount.MountPath)
					return body(c, rootMount)
				})
				if err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *Commands) Mount(src, target string, options ...string) (err error) {
	return c.WithGroupInScopeErr("Mount", func(c *Commands) error {
		mountCmd := c.Cmd("mount", append([]string{src, target}, options...)...)
		err = c.RunCmd(mountCmd)
		if err != nil {
			return err
		}
		return nil
	})
}

func (c *Commands) Unmount(target string, options ...string) (err error) {
	return c.WithGroupInScopeErr("Unmount", func(c *Commands) error {
		mountCmd := c.Cmd("umount", util.RemoveEmptySplits(append(options, target), true)...)
		err = c.RunCmd(mountCmd)
		if err != nil {
			return err
		}
		return nil
	})
}
