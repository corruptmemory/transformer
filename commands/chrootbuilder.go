package commands

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
	"transformer/cmdr"
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

type ChrootProxy struct {
	c                    *Commands
	conn                 net.Conn
	wg                   *sync.WaitGroup
	chrootResultsChannel chan WithChrootResults
	enc                  *gob.Encoder
	dec                  *gob.Decoder
}

func (r *ChrootProxy) RunCmd(command string, args ...string) (stdout string, stderr string, err error) {
	rcCmd := Command{
		Tag:     CommandCli,
		Command: command,
		Args:    args,
	}
	err = r.enc.Encode(rcCmd)
	if err != nil {
		return "", "", err
	}
	r.c.Infof("Command sent to chroot: %s %v", command, args)
	for {
		var reply Reply
		err = r.dec.Decode(&reply)
		if err != nil {
			return "", "", err
		}
		switch reply.Tag {
		case ReplyError:
			return reply.StdOut, reply.StdErr, fmt.Errorf("error from chroot command: %s", reply.Err)
		case ReplyCommandStarted:
			r.c.Infof("Command started in chroot")
			// Do nothing at the moment
		case ReplyCommandCompleted:
			return reply.StdOut, reply.StdErr, nil
		default:
			return "", "", fmt.Errorf("unexpected reply tag: %v", reply.Tag)
		}
	}
}

func (r *ChrootProxy) terminate() (err error) {
	rcCmd := Command{
		Tag: CommandTerminate,
	}
	err = r.enc.Encode(rcCmd)
	if err != nil {
		return err
	}
	err = r.conn.Close()
	if err != nil {
		return err
	}
	for c := range r.chrootResultsChannel {
		switch c.Tag {
		case WCRError:
			r.c.Errorf("Error from chroot: %v", c.Err)
		default:
			// Do nothing
		}
	}
	r.wg.Wait()
	return nil
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
		err = cb.SetupResolvConf(targetPrefix)
		if err != nil {
			return err
		}
		return cb.SetupRemoteControl(targetPrefix)
	})
}

func (cb *ChrootBuilder) SetupRemoteControl(targetPrefix string) (err error) {
	return cb.c.WithGroupInScopeErr("ChrootBuilder.SetupRemoteControl", func(c *Commands) error {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("exe: could get the path to the current executable: %v", err)
		}
		src := filepath.Dir(exe)
		dest := filepath.Join(targetPrefix, "/tmp/remote-control")
		md := cb.c.Cmd("mkdir", "-p", dest)
		err = cb.c.RunCmd(md)
		if err != nil {
			return fmt.Errorf("mkdir: failed to create directory '%s': %v", dest, err)
		}
		return cb.Mount(src, dest, "-o", "bind")
	})
}

type WithChrootResultsTag int

const (
	WCRError WithChrootResultsTag = iota
	WCRRemoteControlRunning
)

type WithChrootResults struct {
	Tag WithChrootResultsTag
	Err error
}

func (cb *ChrootBuilder) WithChrootNoMount(targetDir string, body func(proxy *ChrootProxy) error) (err error) {
	wg := &sync.WaitGroup{}
	chrootResultsChannel := make(chan WithChrootResults, 100)
	chrootRunner := func() {
		var err error
		defer func() {
			if err != nil {
				chrootResultsChannel <- WithChrootResults{Tag: WCRError, Err: err}
			}
			close(chrootResultsChannel)
			wg.Done()
		}()
		err = cb.c.WithGroupInScopeErr("ChrootBuilder.WithChroot", func(c *Commands) (err error) {
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
			err = cb.Setup(targetDir)
			if err != nil {
				return err
			}

			c.Infof("Chrooted into %s", targetDir)
			cmd := c.Cmd("chroot", "/mnt", "bash", "-c", "source /etc/profile; cd /; /tmp/remote-control/upgrader remote-control --socket /tmp/remote-control/remote-control.sock")
			stdOutReader, stdOutWriter, err := os.Pipe()
			if err != nil {
				return err
			}
			stdErrReader, stdErrWriter, err := os.Pipe()
			if err != nil {
				stdOutWriter.Close()
				stdOutReader.Close()
				return err
			}
			cmd.SetStdout(stdOutWriter)
			cmd.SetStderr(stdErrWriter)
			go func() {
				defer func() {
					stdOutWriter.Close()
					stdOutReader.Close()
				}()
				io.Copy(os.Stdout, stdOutReader)
			}()
			go func() {
				defer func() {
					stdErrWriter.Close()
					stdErrReader.Close()
				}()
				io.Copy(os.Stderr, stdErrReader)
			}()

			asyncCmd := c.RunCmdAsync(cmd)
			for c := range asyncCmd {
				switch c.Tag {
				case cmdr.ARStarted:
					cb.c.Infof("Chroot running...")
					chrootResultsChannel <- WithChrootResults{Tag: WCRRemoteControlRunning}
				case cmdr.ARError:
					cb.c.Errorf("Got error: %v", c.Error)
					if err == nil {
						err = c.Error
					}
				case cmdr.ARCompleted:
					cb.c.Infof("Chroot finished...")
				}
			}
			return err
		})
	}
	wg.Add(1)
	go chrootRunner()
resultChannelLoop:
	for c := range chrootResultsChannel {
		switch c.Tag {
		case WCRError:
			err = c.Err
			return
		case WCRRemoteControlRunning:
			break resultChannelLoop
		}
	}
	for {
		cb.c.Infof("Looking for remote-control.sock...")
		e, f, _, err := util.WhatsThere("./remote-control.sock")
		if err != nil {
			cb.c.Errorf("Error checking for remote-control.sock: %v", err)
			return err
		}
		if e && f {
			cb.c.Infof("Found remote-control.sock")
			conn, err := net.Dial("unix", "./remote-control.sock")
			cb.c.Infof("Connected remote-control.sock")
			if err != nil {
				cb.c.Errorf("Error connecting to remote-control.sock: %v", err)
				return err
			}
			proxy := &ChrootProxy{
				c:                    cb.c,
				conn:                 conn,
				wg:                   wg,
				chrootResultsChannel: chrootResultsChannel,
				enc:                  gob.NewEncoder(conn),
				dec:                  gob.NewDecoder(conn),
			}
			defer func() {
				terminateErr := proxy.terminate()
				cb.c.Errorf("Error terminating chroot proxy: %v", terminateErr)
				os.Remove("./remote-control.sock")
			}()
			return body(proxy)
		}
		time.Sleep(1 * time.Second)
	}

}

func (cb *ChrootBuilder) WithChroot(device, targetDir string, body func(proxy *ChrootProxy) error) (err error) {
	wg := &sync.WaitGroup{}
	chrootResultsChannel := make(chan WithChrootResults, 100)
	chrootRunner := func() {
		var err error
		defer func() {
			if err != nil {
				chrootResultsChannel <- WithChrootResults{Tag: WCRError, Err: err}
			}
			close(chrootResultsChannel)
			wg.Done()
		}()
		err = cb.c.WithGroupInScopeErr("ChrootBuilder.WithChroot", func(c *Commands) (err error) {
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

				c.Infof("Chrooted into %s", device)
				cmd := c.Cmd("chroot", "/mnt", "bash", "-c", "source /etc/profile; cd /; /tmp/remote-control/upgrader remote-control --socket /tmp/remote-control/remote-control.sock")
				stdOutReader, stdOutWriter, err := os.Pipe()
				if err != nil {
					return err
				}
				stdErrReader, stdErrWriter, err := os.Pipe()
				if err != nil {
					stdOutWriter.Close()
					stdOutReader.Close()
					return err
				}
				cmd.SetStdout(stdOutWriter)
				cmd.SetStderr(stdErrWriter)
				go func() {
					defer func() {
						stdOutWriter.Close()
						stdOutReader.Close()
					}()
					io.Copy(os.Stdout, stdOutReader)
				}()
				go func() {
					defer func() {
						stdErrWriter.Close()
						stdErrReader.Close()
					}()
					io.Copy(os.Stderr, stdErrReader)
				}()

				asyncCmd := c.RunCmdAsync(cmd)
				for c := range asyncCmd {
					switch c.Tag {
					case cmdr.ARStarted:
						cb.c.Infof("Chroot running...")
						chrootResultsChannel <- WithChrootResults{Tag: WCRRemoteControlRunning}
					case cmdr.ARError:
						cb.c.Errorf("Got error: %v", c.Error)
						if err == nil {
							err = c.Error
						}
					case cmdr.ARCompleted:
						cb.c.Infof("Chroot finished...")
					}
				}
				return err
			})
		})
	}
	wg.Add(1)
	go chrootRunner()
resultChannelLoop:
	for c := range chrootResultsChannel {
		switch c.Tag {
		case WCRError:
			err = c.Err
			return
		case WCRRemoteControlRunning:
			break resultChannelLoop
		}
	}
	for {
		cb.c.Infof("Looking for remote-control.sock...")
		e, f, _, err := util.WhatsThere("./remote-control.sock")
		if err != nil {
			cb.c.Errorf("Error checking for remote-control.sock: %v", err)
			return err
		}
		if e && f {
			cb.c.Infof("Found remote-control.sock")
			conn, err := net.Dial("unix", "./remote-control.sock")
			cb.c.Infof("Connected remote-control.sock")
			if err != nil {
				cb.c.Errorf("Error connecting to remote-control.sock: %v", err)
				return err
			}
			proxy := &ChrootProxy{
				c:                    cb.c,
				conn:                 conn,
				wg:                   wg,
				chrootResultsChannel: chrootResultsChannel,
				enc:                  gob.NewEncoder(conn),
				dec:                  gob.NewDecoder(conn),
			}
			defer func() {
				terinateErr := proxy.terminate()
				cb.c.Errorf("Error terminating chroot proxy: %v", terinateErr)
				os.Remove("./remote-control.sock")
			}()
			return body(proxy)
		}
		time.Sleep(1 * time.Second)
	}

}
