package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jessevdk/go-flags"
	"io"
	"os"
	"path/filepath"
	"time"
	"transformer/commands"
	"transformer/fstabutils"
	"transformer/sfdiskutils"
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
	var originalFSTabTemplate fstabutils.FstabProcessResult
	c.Infof("Backing up the OS/EPIC partition")
	err = c.WithTempMountErr(osDevice, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("OS/EPIC device mounted at %s", mount.MountPath)
		c.Infof("Capturing fstab details")
		fstabFile, err := os.Open(filepath.Join(mount.MountPath, "etc/fstab"))
		if err != nil {
			c.Errorf("Error opening fstab file: %v", err)
			return err
		}
		defer fstabFile.Close()
		fstabData, err := io.ReadAll(fstabFile)
		if err != nil {
			c.Errorf("Error reading fstab file: %v", err)
		}
		originalFSTabTemplate, err = fstabutils.ProcessFstab(string(fstabData), true)
		if err != nil {
			c.Errorf("Error extracting fstab data: %v", err)
			c.Errorf("Input: %s", string(fstabData))
			return err
		}
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
	c.Infof("Getting `sfdisk` partition layout")
	sfdisk, err := c.SFDiskData(da.DevicePath)
	if err != nil {
		c.Errorf("Error getting `sfdisk` partition layout: %v", err)
		return
	}
	var sfdiskPart sfdiskutils.SFPartitionInfo
	for _, p := range sfdisk.Partitions {
		if p.Partition == osDevice {
			sfdiskPart = p
		}
	}
	if sfdiskPart.Partition == "" {
		c.Errorf("Error: could not find the OS partition in the `sfdisk` layout")
		c.Errorf("Given: %+v", sfdisk)
		return
	}
	pn, err := sfdiskPart.PartitionNumber()
	if err != nil {
		c.Errorf("Error getting partition number: %v from %s", err, sfdiskPart.Partition)
		return
	}
	c.Infof("--- Original layout ---\n%s\n---------------", sfdisk.String())
	c.Infof("Splitting the OS partition[%d]: %s", pn, osDevice)
	newUUIDString := uuid.New().String()
	err = sfdisk.SplitPartitionEqually(pn, newUUIDString, "", "")
	if err != nil {
		c.Errorf("Failed to split partition %d: %v", pn, err)
		c.Errorf("Input: %s", sfdisk.String())
		return
	}
	c.Infof("--- New layout ---\n%s\n---------------", sfdisk.String())
	c.Infof("Writing new partition layout")
	err = c.SFDiskApply(da.DevicePath, sfdisk)
	if err != nil {
		c.Errorf("Error applying new disk layout: %v", err)
		return
	}
	c.Infof("Syncing partiton info...")
	partProbeCmd := c.Cmd("partprobe", da.DevicePath)
	err = partProbeCmd.Run()
	if err != nil {
		c.Errorf("Error running partprobe: %v", err)
		c.Errorf("STDERR: %s", partProbeCmd.GetStderrAsString())
		c.Errorf("STDOUT: %s", partProbeCmd.GetStdoutAsString())
		return
	}
	c.Infof("Pausing for 2 seconds to allow the system to catch up")
	time.Sleep(2 * time.Second)
	originalPartition, err := sfdisk.FindPartition(pn)
	if err != nil {
		c.Errorf("Error: could not look up the original partition: %v given: %d", err, pn)
		return
	}
	var newPartition sfdiskutils.SFPartitionInfo
	for _, p := range sfdisk.Partitions {
		if p.PartUUID == newUUIDString {
			newPartition = p
		}
	}
	if newPartition.Partition == "" {
		c.Errorf("Error: could not find the new partition in the `sfdisk` layout.  Looking for %s", newUUIDString)
		c.Errorf("Given: %s", sfdisk.String())
		return
	}
	c.Infof("Formatting updated original OS partition: %s", originalPartition.String())
	err = c.Mkfs(originalPartition.Partition)
	if err != nil {
		c.Errorf("Error formatting original partition: %v", err)
		return
	}
	c.Infof("Formatting new OS partition: %s", newPartition.String())
	err = c.Mkfs(newPartition.Partition)
	if err != nil {
		c.Errorf("Error formatting new partition: %v", err)
		return
	}
	c.Infof("Restoring original OS/EPIC partition from backup")
	err = c.WithTempMountErr(originalPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("OS/EPIC device mounted at %s", mount.MountPath)
		path, err := c.ExpandPath("./backup")
		if err != nil {
			return fmt.Errorf("error expanding path(./backup): %v", err)
		}

		c.Infof("Restoring the OS/EPIC partition")
		return c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
			events := c.ExtractTarball(context.Background(), path+"/os.tar.zstd", ".")
			for e := range events {
				switch e.Tag {
				case commands.ETTarballFileSize:
					c.Infof("Tarball file size: %d", e.TotalFileSize)
				case commands.ETReadingFiles:
					c.Infof("Restoring backup[%d%%]: %s -> %s", e.ProgressPercent(), e.Tarball, e.TargetDir)
				case commands.ETSuccess:
					c.Info("Restore complete")
				case commands.ETError:
					c.Errorf("Error restoring backup: %v", e.Error)
					return e.Error
				}
			}
			return nil
		})
	})
	if err != nil {
		c.Errorf("Error restoring OS/EPIC partition: %v", err)
		return
	}
	c.Infof("Installing new OS/EPIC partition")
	err = c.WithTempMountErr(newPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("NEW OS/EPIC device mounted at %s", mount.MountPath)
		path, err := c.ExpandPath("./new-image")
		if err != nil {
			return fmt.Errorf("error expanding path(./new-image): %v", err)
		}

		c.Infof("Expanding the NEW OS/EPIC partition")
		err = c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
			events := c.ExtractTarball(context.Background(), path+"/ubuntu2404.tar.zstd", ".")
			for e := range events {
				switch e.Tag {
				case commands.ETTarballFileSize:
					c.Infof("Tarball file size: %d", e.TotalFileSize)
				case commands.ETReadingFiles:
					c.Infof("Restoring backup[%d%%]: %s -> %s", e.ProgressPercent(), e.Tarball, e.TargetDir)
				case commands.ETSuccess:
					c.Info("Restore complete")
				case commands.ETError:
					c.Errorf("Error restoring backup: %v", e.Error)
					return e.Error
				}
			}
			return nil
		})
		if err != nil {
			c.Errorf("Error expanding new OS/EPIC partition: %v", err)
			return err
		}
		c.Infof("Capturing fstab details")
		fstabFile, err := os.Open(filepath.Join(mount.MountPath, "etc/fstab"))
		if err != nil {
			c.Errorf("Error opening fstab file: %v", err)
			return err
		}
		defer fstabFile.Close()
		fstabData, err := io.ReadAll(fstabFile)
		if err != nil {
			c.Errorf("Error reading fstab file: %v", err)
			return err
		}
		newFSTabTemplate, err := fstabutils.ProcessFstab(string(fstabData), false)
		if err != nil {
			c.Errorf("Error extracting fstab data: %v", err)
			c.Errorf("Input: %s", string(fstabData))
			return err
		}
		c.Infof("Updating new fstab with deatils form the original fstab")
		newInputs := fstabutils.FstabTemplateInput{
			RootDevice:       originalFSTabTemplate.OriginalRootDevice,
			BootDevice:       originalFSTabTemplate.OriginalBootDevice,
			EfiDevice:        originalFSTabTemplate.OriginalEfiDevice,
			RecordingsDevice: originalFSTabTemplate.OriginalRecordingsDevice,
		}
		newFstab, err := newFSTabTemplate.Execute(newInputs)
		if err != nil {
			c.Errorf("Error updating new fstab: %v", err)
		}
		c.Infof("\n---- New fstab ----\n%s\n-------------------\n", newFstab)
		c.Infof("Writing new fstab")
		fstabFile, err = os.Create(filepath.Join(mount.MountPath, "etc/fstab"))
		if err != nil {
			c.Errorf("Error creating fstab file: %v", err)
			return err
		}
		defer fstabFile.Close()
		_, err = fstabFile.WriteString(newFstab)
		if err != nil {
			c.Errorf("Error writing fstab file: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		c.Errorf("Error installing new OS/EPIC partition: %v", err)
		return
	}
	c.Infof("Installing new OS boot images")
	c.Infof("Creating tmp mount point")
	_, err = c.MkdirAll("/tmp/boot", 0755)
	if err != nil {
		c.Errorf("Error creating /tmp/boot: %v", err)
		return
	}
	path, err := c.ExpandPath("./new-image")
	if err != nil {
		c.Errorf("Error expanding path(./new-image): %v", err)
		return
	}
	err = c.WithCd("/tmp/boot", func(c *commands.Commands) error {

		c.Infof("Expanding the NEW boot partition")
		events := c.ExtractTarball(context.Background(), path+"/grub.tar.zstd", ".")
		for e := range events {
			switch e.Tag {
			case commands.ETTarballFileSize:
				c.Infof("Tarball file size: %d", e.TotalFileSize)
			case commands.ETReadingFiles:
				c.Infof("Restoring backup[%d%%]: %s -> %s", e.ProgressPercent(), e.Tarball, e.TargetDir)
			case commands.ETSuccess:
				c.Info("Restore complete")
			case commands.ETError:
				c.Errorf("Error restoring backup: %v", e.Error)
				return e.Error
			}
		}
		c.Infof("Mounting the boot partition")
		return c.WithTempMountErr(grubDevice, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
			c.Infof("GRUB device mounted at %s", mount.MountPath)
			c.Infof("Copying boot files")
			cwd, err := c.ExpandPath(".")
			if err != nil {
				c.Errorf("Error expanding path(.): %v", err)
				return err
			}
			err = c.CopyBootImages(cwd, mount.MountPath)
			if err != nil {
				c.Errorf("Error copying files: %v", err)
				return err
			}
			return nil
		})
	})
	if err != nil {
		c.Errorf("Error installing new boot images: %v", err)
		return
	}
	err = os.RemoveAll("/tmp/boot")
	if err != nil {
		c.Errorf("Error cleaning up /tmp/boot: %v", err)
		return
	}

	c.Infof("Updating GRUB configuration")
	err = c.WithTempMountErr(originalPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("Original OS/EPIC device mounted at %s", mount.MountPath)
		originalMountPath := mount.MountPath
		return c.WithTempMountErr(grubDevice, "/mnt/boot", func(c *commands.Commands, mount commands.MountDetails) error {
			c.Infof("Original GRUB device mounted at %s", mount.MountPath)
			return c.WithTempMountErr(bootDevice, "/mnt/boot/efi", func(c *commands.Commands, mount commands.MountDetails) error {
				c.Infof("Original EFI device mounted at %s", mount.MountPath)
				c.Infof("Entering Chroot...")
				chr := c.ChrootBuilder()
				proxy, err := chr.WithChrootNoMount(originalMountPath)
				if err != nil {
					c.Errorf("Error entering chroot on %s: %v", originalMountPath, err)
				}
				c.Infof("Running update-grub")
				so, se, err := proxy.RunCmd("update-grub")
				if err != nil {
					c.Errorf("Error running update-grub: %v", err)
					c.Errorf("STDOUT: %s", so)
					c.Errorf("STDERR: %s", se)
					return err
				}
				c.Infof("STDOUT: %s", so)
				c.Infof("STDERR: %s", se)
				c.Infof("Tearing down chroot...")
				err = proxy.Terminate()
				if err != nil {
					c.Errorf("Error terminating chroot: %v", err)
					return err
				}
				return nil
			})
		})
	})
	if err != nil {
		c.Errorf("Error running grup update: %v", err)
		return
	}

	//c.Infof("Testing Chroot...")
	//chr := c.ChrootBuilder()
	//proxy, err := chr.WithChroot(epicDevice, "/mnt")
	//if err != nil {
	//	c.Errorf("Error testing Chroot: %v", err)
	//	return
	//}
	//c.Infof("Running test commands in chroot")
	//so, se, err := proxy.RunCmd("ls", "-l", ".")
	//if err != nil {
	//	c.Errorf("Error running command: %v", err)
	//	return
	//}
	//c.Infof("STDOUT: %s", so)
	//c.Infof("STDERR: %s", se)
	//err = proxy.Terminate()
	//if err != nil {
	//	c.Errorf("Error terminating chroot: %v", err)
	//}
	//c.Infof("Chroot test successful")
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
