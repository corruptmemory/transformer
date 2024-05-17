package main

import (
	"context"

	"transformer/commands"
)

func main() {
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
	var hasEPIC bool
	var epicDevice string
	for _, p := range pis {
		if p.HasEPICApplication {
			hasEPIC = true
			epicDevice = p.PartitionDevice
			break
		}
	}
	if !hasEPIC {
		c.Errorf("No EPIC application found")
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
		if err != nil {
			c.Errorf("Error creating backup directory: %v", err)
			return err
		}
		c.Infof("Backing up the OS/EPIC partition")
		events := c.CreateTarball(context.Background(), mount.MountPath, path+"/os.tar.zstd")
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
	if err != nil {
		c.Errorf("Error making OS/EPIC backup: %v", err)
		return
	}
	c.Infof("Backing up the OS/EPIC partition")

}
