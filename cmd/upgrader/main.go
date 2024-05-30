package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/jessevdk/go-flags"
	"os"
	"transformer/commands"
)

type Upgrader struct {
}

type RemoteControl struct {
	Socket string `short:"s" long:"socket" description:"Socket to connect to"`
}

type Args struct {
	Upgrader      Upgrader      `command:"upgrade" description:"Upgrade the system"`
	RemoteControl RemoteControl `command:"remote-control" description:"Control the system remotely"`
}

func upgrader() {
	c, err := commands.DefaultCommands(false)
	if err != nil {
		panic(err)
	}
	c.Infof("Audio Enhancement OS Upgrader")
	c.Infof("--------------------------------")
	c.Infof("Finding disk array")
	// TODO: Support finding multiple disk arrays.  When we do that we can use the partition info to detect what
	//       disk array we want to work with.
	da, err := c.FindDiskArray(commands.DefaultFindDiskArraySources)
	if err != nil {
		c.Errorf("Error finding disk array: %v", err)
		return
	}
	c.Infof("Getting disk layout for %s", da.DevicePath)
	pis, err := c.FindPartitionInfo(da.DevicePath)
	if err != nil {
		c.Errorf("Error finding partition info: %v", err)
		return
	}
	c.Infof("Ensure that we are looking at a disk with Ubuntu 20.04 on it")
	var validOS bool
	var osDevice string
	for _, p := range pis {
		if p.HasOSRelease {
			if p.OSRelease.VersionID == "20.04" {
				validOS = true
				osDevice = p.PartitionDevice
				break
			}
		}
	}
	if !validOS {
		c.Errorf("No Ubuntu 20.04 OS found")
		return
	}
	c.Infof("Ensure that we have a disk with the EPIC applicaton on it")
	var grubDevice string
	var epicDevice string
	var bootDevice string
	for _, p := range pis {
		if p.HasEPICApplication {
			epicDevice = p.PartitionDevice
			break
		}
		if p.HasGrubConfig {
			grubDevice = p.PartitionDevice
		}
		if p.Boot {
			bootDevice = p.PartitionDevice
		}
	}
	if epicDevice == "" {
		c.Errorf("No EPIC application found")
		return
	}
	if grubDevice == "" {
		c.Errorf("No Grub configuration found")
		return
	}
	if bootDevice == "" {
		c.Errorf("No boot partition found")
		return
	}
	c.Infof("Ensure the OS device and the EPIC device are the same")
	if osDevice != epicDevice {
		c.Errorf("OS device and EPIC device are not the same")
		return
	}
	c.Infof("Backing up the OS/EPIC partition")
	err = c.WithTempMountErr(osDevice, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("OS/EPIC device mounted at %s", mount.MountPath)
		path, err := c.MkdirAll("./backup", 0755)
		c.Infof("Backup directory: %s", path)
		if err != nil {
			c.Errorf("Error creating backup directory: %v", err)
			return err
		}
		c.Infof("Backing up the OS/EPIC partition")
		return c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
			events := c.CreateTarball(context.Background(), ".", path+"/os.tar.zstd")
			for e := range events {
				switch e.Tag {
				case commands.CTComputingTotalFileSize:
					c.Info("Computing total file size")
				case commands.CTTotalFileSizeComputed:
					c.Infof("Total file size: %d", e.TotalFileSize)
				case commands.CTWritingFiles:
					c.Infof("Building backup[%d%%]: %s -> %s", e.ProgressPercent(), e.SourceDir, e.Tarball)
				case commands.CTSuccess:
					c.Info("Backup complete")
				case commands.CTError:
					c.Errorf("Error building backup: %v", e.Error)
					return e.Error
				}
			}
			return nil
		})
	})
	if err != nil {
		c.Errorf("Error making OS/EPIC backup: %v", err)
		return
	}
	c.Infof("Backing up the GRUB partition")
	err = c.WithTempMountErr(grubDevice, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("GRUB device mounted at %s", mount.MountPath)
		path, err := c.MkdirAll("./backup", 0755)
		c.Infof("Backup directory: %s", path)
		if err != nil {
			c.Errorf("Error creating backup directory: %v", err)
			return err
		}
		c.Infof("Backing up the GURB partition")
		return c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
			events := c.CreateTarball(context.Background(), ".", path+"/grub.tar.zstd")
			for e := range events {
				switch e.Tag {
				case commands.CTComputingTotalFileSize:
					c.Info("Computing total file size")
				case commands.CTTotalFileSizeComputed:
					c.Infof("Total file size: %d", e.TotalFileSize)
				case commands.CTWritingFiles:
					c.Infof("Building backup[%d%%]: %s -> %s", e.ProgressPercent(), e.SourceDir, e.Tarball)
				case commands.CTSuccess:
					c.Info("Backup complete")
				case commands.CTError:
					c.Errorf("Error building backup: %v", e.Error)
					return e.Error
				}
			}
			return nil
		})
	})
	if err != nil {
		c.Errorf("Error making GRUB backup: %v", err)
		return
	}
	if grubDevice != bootDevice {
		c.Infof("Backing up the BOOT partition")
		err = c.WithTempMountErr(bootDevice, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
			c.Infof("BOOT device mounted at %s", mount.MountPath)
			path, err := c.MkdirAll("./backup", 0755)
			c.Infof("Backup directory: %s", path)
			if err != nil {
				c.Errorf("Error creating backup directory: %v", err)
				return err
			}
			c.Infof("Backing up the BOOT partition")
			return c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
				events := c.CreateTarball(context.Background(), ".", path+"/boot.tar.zstd")
				for e := range events {
					switch e.Tag {
					case commands.CTComputingTotalFileSize:
						c.Info("Computing total file size")
					case commands.CTTotalFileSizeComputed:
						c.Infof("Total file size: %d", e.TotalFileSize)
					case commands.CTWritingFiles:
						c.Infof("Building backup[%d%%]: %s -> %s", e.ProgressPercent(), e.SourceDir, e.Tarball)
					case commands.CTSuccess:
						c.Info("Backup complete")
					case commands.CTError:
						c.Errorf("Error building backup: %v", e.Error)
						return e.Error
					}
				}
				return nil
			})
		})
		if err != nil {
			c.Errorf("Error making BOOT backup: %v", err)
			return
		}
	}
	c.Infof("All backups completed... ready to upgrade")
	c.Infof("Testing Chroot...")
	chr := c.ChrootBuilder()
	proxy, err := chr.WithChroot(epicDevice, "/mnt")
	if err != nil {
		c.Errorf("Error testing Chroot: %v", err)
		return
	}
	c.Infof("Running test commands in chroot")
	so, se, err := proxy.RunCmd("ls", "-l", ".")
	if err != nil {
		c.Errorf("Error running command: %v", err)
		return
	}
	c.Infof("STDOUT: %s", so)
	c.Infof("STDERR: %s", se)
	err = proxy.Terminate()
	if err != nil {
		c.Errorf("Error terminating chroot: %v", err)
	}
	c.Infof("Chroot test successful")
}

func remoteControl(args RemoteControl) {
	c, err := commands.DefaultCommands(false)
	if err != nil {
		panic(err)
	}
	err = c.StartRemoteControlSession(args.Socket)
	if err != nil {
		c.Errorf("Failed to start remote control session: %v", err)
		return
	}
	c.Infof("Remote control session finished")
}

func main() {
	args := Args{}
	parser := flags.NewParser(&args, flags.Default)
	if _, err := parser.Parse(); err != nil {
		switch flagsErr := err.(type) {
		case *flags.Error:
			if errors.Is(flagsErr.Type, flags.ErrHelp) {
				return
			}
			fmt.Printf("Error with command line argument: %v", err)
		default:
			fmt.Printf("Error with command line argument: %v", err)
		}
		os.Exit(1)
	}

	switch parser.Active.Name {
	case "upgrade":
		upgrader()
	case "remote-control":
		remoteControl(args.RemoteControl)
	}

}
