package commands

import (
	"errors"
	"fmt"
	"path/filepath"
	"transformer/util"
)

type ChrootBuilder struct {
	c           *Commands
	mountPoints []string
}

func (c *Commands) ChrootBuilder() *ChrootBuilder {
	return &ChrootBuilder{c: c}
}

type MaybeMount interface {
	Src() string
	Target() string
	Options() []string
	ShouldMount() bool
}

type MountWithCondition struct {
	src       string
	target    string
	options   []string
	condition func() bool
}

func (d MountWithCondition) Src() string {
	return d.src
}

func (d MountWithCondition) Target() string {
	return d.target
}

func (d MountWithCondition) Options() []string {
	return d.options
}

func (d MountWithCondition) ShouldMount() bool {
	if d.condition == nil {
		return true
	}
	return d.condition()
}

func DefiniteMount(src, target string, options ...string) MaybeMount {
	return MountWithCondition{
		src:     src,
		target:  target,
		options: options,
	}
}

func ConditionalMount(src, target string, options ...string) func(condition func() bool) MaybeMount {
	return func(condition func() bool) MaybeMount {
		return MountWithCondition{
			src:       src,
			target:    target,
			options:   options,
			condition: condition,
		}
	}
}

func (cb *ChrootBuilder) Mount(src, target string, options ...string) (err error) {
	return cb.c.WithGroupInScopeErr("ChrootBuilder.Mount", func(c *Commands) error {
		err = cb.c.Mount(src, target, options...)
		if err != nil {
			return err
		}
		cb.mountPoints = append(cb.mountPoints, target)
		return nil
	})
}

func (cb *ChrootBuilder) MountAll(mounts []MaybeMount) (err error) {
	return cb.c.WithGroupInScopeErr("ChrootBuilder.MountAll", func(c *Commands) error {
		for _, m := range mounts {
			if m.ShouldMount() {
				err = cb.Mount(m.Src(), m.Target(), m.Options()...)
				if err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (cb *ChrootBuilder) Teardown() (err error) {
	return cb.c.WithGroupInScopeErr("ChrootBuilder.Teardown", func(c *Commands) error {
		me := util.NewMultiError()
		for i := len(cb.mountPoints) - 1; i >= 0; i-- {
			err = cb.c.Unmount(cb.mountPoints[i])
			if err != nil {
				me.Add(fmt.Errorf("error unmounting %s: %v", cb.mountPoints[i], err))
			}
		}
		cb.mountPoints = nil
		if me.HasErrors() {
			return me
		}
		return nil
	})
}

func (cb *ChrootBuilder) SetupResolvConf(targetPrefix string) (err error) {
	return cb.c.WithGroupInScopeErr("ChrootBuilder.SetupResolvConf", func(c *Commands) error {
		src, err := util.ResolveSymlink("/etc/resolv.conf")
		if err != nil {
			return fmt.Errorf("src: could not resolve symlink '/etc/resolv.conf': %v", err)
		}
		if !util.IsFile(src) {
			return fmt.Errorf("src: '%s' is not a file", src)
		}
		dest := filepath.Join(targetPrefix, "etc/resolv.conf")
		dest, err = util.ResolveSymlink(dest)
		if err != nil {
			var lstatErr *util.LStatFileNotFoundError
			if errors.As(err, &lstatErr) {
				md := cb.c.Cmd("mkdir", "-p", filepath.Dir(lstatErr.Path))
				err = cb.c.RunCmd(md)
				if err != nil {
					return fmt.Errorf("dest: could not create directory '%s': %v", filepath.Dir(lstatErr.Path), err)
				}
				touch := cb.c.Cmd("touch", lstatErr.Path)
				err = cb.c.RunCmd(touch)
				if err != nil {
					return fmt.Errorf("dest: could not create file '%s': %v", dest, err)
				}
				dest = lstatErr.Path
			} else {
				return fmt.Errorf("dest: could not resolve symlink '%s': %v", dest, err)
			}
		}
		if !util.IsFile(dest) {
			return fmt.Errorf("dest: '%s' is not a file", dest)
		}
		return cb.Mount(src, dest, "-o", "bind")
	})
}

func (cb *ChrootBuilder) Setup(targetPrefix string) (err error) {
	defer func() {
		if err != nil {
			err1 := cb.Teardown()
			if err1 != nil {
				err = fmt.Errorf("multiple errors: %v, %v", err, err1)
			}
		}
	}()
	return cb.c.WithGroupInScopeErr("ChrootBuilder.Setup", func(c *Commands) (err error) {
		err = cb.MountAll([]MaybeMount{
			DefiniteMount("proc", filepath.Join(targetPrefix, "proc"), "-t", "proc", "-o", "nosuid,noexec,nodev"),
			DefiniteMount("sys", filepath.Join(targetPrefix, "sys"), "-t", "sysfs", "-o", "nosuid,noexec,nodev,ro"),
			ConditionalMount("efivarfs", filepath.Join(targetPrefix, "sys/firmware/efi/efivars"), "-t", "efivarfs", "-o", "nosuid,noexec,nodev")(func() bool {
				e, d, _, err := util.WhatsThere(filepath.Join(targetPrefix, "sys/firmware/efi/efivars"))
				if err != nil {
					return false
				}
				return e && d
			}),
			DefiniteMount("udev", filepath.Join(targetPrefix, "dev"), "-t", "devtmpfs", "-o", "mode=0755,nosuid"),
			DefiniteMount("devpts", filepath.Join(targetPrefix, "dev/pts"), "-t", "devpts", "-o", "mode=0620,gid=5,nosuid,noexec"),
			DefiniteMount("shm", filepath.Join(targetPrefix, "dev/shm"), "-t", "tmpfs", "-o", "mode=1777,nosuid,nodev"),
			DefiniteMount("run", filepath.Join(targetPrefix, "run"), "-t", "tmpfs", "-o", "nosuid,nodev,mode=0755"),
			DefiniteMount("tmp", filepath.Join(targetPrefix, "tmp"), "-t", "tmpfs", "-o", "mode=1777,strictatime,nodev,nosuid"),
		})
		if err != nil {
			return err
		}
		return cb.SetupResolvConf(targetPrefix)
	})
}

func (cb *ChrootBuilder) WithChroot(device, targetDir string, body func(c *Commands) error) (err error) {
	return cb.c.WithGroupInScopeErr("ChrootBuilder.WithChroot", func(c *Commands) (err error) {
		return cb.c.WithTempMountErr(device, targetDir, func(c *Commands, md MountDetails) (err error) {
			defer func() {
				err1 := cb.Teardown()
				if err1 != nil {
					if err != nil {
						err = fmt.Errorf("multiple errors: %v, %v", err, err1)
					} else {
						err = err1
					}
				}
			}()
			err = cb.Setup(md.MountPath)
			if err != nil {
				return err
			}
			return body(c)
		})
	})
}
