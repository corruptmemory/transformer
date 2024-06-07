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
	"transformer/blkidutils"
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
	var grubPartition commands.PartitionInfo
	var epicDevice string
	var epicPartition commands.PartitionInfo
	var efiBootDevice string
	var efiBootPartition commands.PartitionInfo
	for _, p := range pis {
		if p.HasEPICApplication {
			epicDevice = p.PartitionDevice
			epicPartition = p
			break
		}
		if p.HasGrubConfig {
			grubDevice = p.PartitionDevice
			grubPartition = p
		}
		if p.Boot {
			efiBootDevice = p.PartitionDevice
			efiBootPartition = p
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
	if efiBootDevice == "" {
		c.Errorf("No boot partition found")
		return
	}
	if efiBootPartition.EFIGrubTemplate == nil {
		c.Errorf("Could not find the GRUB configuration template")
		c.Infof("Details: %+v", efiBootPartition)
		return
	}
	c.Infof("Ensuring that the OS and GRUB partitions have enough room to split")
	c.Infof("  > Used space on OS partition: %.2f%%", epicPartition.DiskUsage.PercentageUsed())
	c.Infof("  > Used space on GRUB partition: %.2f%%", grubPartition.DiskUsage.PercentageUsed())
	if epicPartition.DiskUsage.PercentageUsed() >= 50 {
		c.Errorf("Insufficient space on the OS partition to split")
		return
	}
	if grubPartition.DiskUsage.PercentageUsed() >= 50 {
		c.Errorf("Insufficient space on the GRUB partition to split")
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
	if grubDevice != efiBootDevice {
		c.Infof("Backing up the BOOT partition")
		err = c.WithTempMountErr(efiBootDevice, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
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
	var sfdiskOSPart sfdiskutils.SFPartitionInfo
	for _, p := range sfdisk.Partitions {
		if p.Partition == osDevice {
			sfdiskOSPart = p
		}
	}
	if sfdiskOSPart.Partition == "" {
		c.Errorf("Error: could not find the OS partition in the `sfdisk` layout")
		c.Errorf("Given: %+v", sfdisk)
		return
	}
	var sfdiskGRUBPart sfdiskutils.SFPartitionInfo
	for _, p := range sfdisk.Partitions {
		if p.Partition == grubDevice {
			sfdiskGRUBPart = p
		}
	}
	if sfdiskGRUBPart.Partition == "" {
		c.Errorf("Error: could not find the GRUB partition in the `sfdisk` layout")
		c.Errorf("Given: %+v", sfdisk)
		return
	}
	ospn, err := sfdiskOSPart.PartitionNumber()
	if err != nil {
		c.Errorf("Error getting partition number: %v from %s", err, sfdiskOSPart.Partition)
		return
	}
	grubpn, err := sfdiskGRUBPart.PartitionNumber()
	if err != nil {
		c.Errorf("Error getting partition number: %v from %s", err, sfdiskGRUBPart.Partition)
		return
	}
	c.Infof("--- Original layout ---\n%s\n---------------", sfdisk.String())
	c.Infof("Splitting the OS partition[%d]: %s", ospn, osDevice)
	newOSUUIDString := uuid.New().String()
	err = sfdisk.SplitPartitionEqually(ospn, newOSUUIDString, "", "")
	if err != nil {
		c.Errorf("Failed to split partition %d: %v", ospn, err)
		c.Errorf("Input: %s", sfdisk.String())
		return
	}
	c.Infof("Splitting the GRUB partition[%d]: %s", grubpn, osDevice)
	newGRUBUUIDString := uuid.New().String()
	err = sfdisk.SplitPartitionEqually(grubpn, newGRUBUUIDString, "", "")
	if err != nil {
		c.Errorf("Failed to split partition %d: %v", grubpn, err)
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
	err = c.RunCmd(partProbeCmd)
	if err != nil {
		c.Errorf("Error running partprobe: %v", err)
		c.Errorf("STDERR: %s", partProbeCmd.GetStderrAsString())
		c.Errorf("STDOUT: %s", partProbeCmd.GetStdoutAsString())
		return
	}
	c.Infof("Pausing for 2 seconds to allow the system to catch up")
	time.Sleep(2 * time.Second)
	err = sfdisk.BuildByUUIDMapping("/dev/disk/by-id")
	if err != nil {
		c.Errorf("Error: could not get the UUID mappings: %v", err)
		return
	}
	originalOSPartition, err := sfdisk.FindPartition(ospn)
	if err != nil {
		c.Errorf("Error: could not look up the original OS partition: %v given: %d", err, ospn)
		return
	}
	var newOSPartition sfdiskutils.SFPartitionInfo
	for _, p := range sfdisk.Partitions {
		if p.PartUUID == newOSUUIDString {
			newOSPartition = p
		}
	}
	if newOSPartition.Partition == "" {
		c.Errorf("Error: could not find the new OS partition in the `sfdisk` layout.  Looking for %s", newOSUUIDString)
		c.Errorf("Given: %s", sfdisk.String())
		return
	}
	originalGRUBPartition, err := sfdisk.FindPartition(grubpn)
	if err != nil {
		c.Errorf("Error: could not look up the original GRUB partition: %v given: %d", err, grubpn)
		return
	}
	var newGRUBPartition sfdiskutils.SFPartitionInfo
	for _, p := range sfdisk.Partitions {
		if p.PartUUID == newGRUBUUIDString {
			newGRUBPartition = p
		}
	}
	if newGRUBPartition.Partition == "" {
		c.Errorf("Error: could not find the new GRUB partition in the `sfdisk` layout.  Looking for %s", newGRUBPartition)
		c.Errorf("Given: %s", sfdisk.String())
		return
	}
	c.Infof("Formatting updated original OS partition: %s", originalOSPartition.String())
	err = c.Mkfs(originalOSPartition.Partition)
	if err != nil {
		c.Errorf("Error formatting original OS partition: %v", err)
		return
	}
	c.Infof("Formatting new OS partition: %s", newOSPartition.String())
	err = c.Mkfs(newOSPartition.Partition)
	if err != nil {
		c.Errorf("Error formatting new OS partition: %v", err)
		return
	}
	c.Infof("Formatting updated original GRUB partition: %s", originalGRUBPartition.String())
	err = c.Mkfs(originalGRUBPartition.Partition)
	if err != nil {
		c.Errorf("Error formatting original GRUB partition: %v", err)
		return
	}
	c.Infof("Formatting new GRUB partition: %s", newGRUBPartition.String())
	err = c.Mkfs(newGRUBPartition.Partition)
	if err != nil {
		c.Errorf("Error formatting new GRUB partition: %v", err)
		return
	}
	c.Infof("Get updated UUIDs for the new partitions")
	newBlkIDLayout, err := c.BLKIDLayout(da.DevicePath)
	if err != nil {
		c.Errorf("Could not get the updated UUIDs: %v", err)
		return
	}
	var updatedOriginalGRUBPartition blkidutils.BlkIdPartitionInfo
	var updatedNewGRUBPartition blkidutils.BlkIdPartitionInfo
	for _, p := range newBlkIDLayout.Partitions {
		if p.Partition == originalGRUBPartition.Partition {
			updatedOriginalGRUBPartition = p
		}
		if p.Partition == newGRUBPartition.Partition {
			updatedNewGRUBPartition = p
		}
	}
	if updatedOriginalGRUBPartition.Partition == "" {
		c.Errorf("Error, could not find the updated original GRUB partition in the `blkid` layout: %+v", newBlkIDLayout)
		return
	}
	if updatedNewGRUBPartition.Partition == "" {
		c.Errorf("Error, could not find the updated new GRUB partition in the `blkid` layout: %+v", newBlkIDLayout)
		return
	}
	c.Infof("Restoring original OS/EPIC partition from backup")
	err = c.WithTempMountErr(originalOSPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("OS/EPIC device mounted at %s", mount.MountPath)
		path, err := c.ExpandPath("./backup")
		if err != nil {
			return fmt.Errorf("error expanding path(./backup): %v", err)
		}

		c.Infof("Restoring the OS/EPIC partition")
		err = c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
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
		if err != nil {
			c.Errorf("Error restoring original OS/EPIC partition: %v", err)
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
			BootDevice:       originalGRUBPartition.DeviceByUUID,
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
		c.Errorf("Error restoring OS/EPIC partition: %v", err)
		return
	}
	c.Infof("Restoring original GRUB partition from backup")
	err = c.WithTempMountErr(originalGRUBPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("original GRUB device mounted at %s", mount.MountPath)
		path, err := c.ExpandPath("./backup")
		if err != nil {
			return fmt.Errorf("error expanding path(./backup): %v", err)
		}

		c.Infof("Restoring the original GRUB partition")
		err = c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
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
			return nil
		})
		if err != nil {
			c.Errorf("Error restoring original GRUB partition: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		c.Errorf("Error restoring original GRUB partition: %v", err)
		return
	}
	c.Infof("Installing new OS/EPIC partition")
	err = c.WithTempMountErr(newOSPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
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
			RootDevice:       newOSPartition.DeviceByUUID,
			BootDevice:       newGRUBPartition.DeviceByUUID,
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
	c.Infof("Restoring new GRUB partition from backup")
	err = c.WithTempMountErr(newGRUBPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("new GRUB device mounted at %s", mount.MountPath)
		path, err := c.ExpandPath("./new-image")
		if err != nil {
			return fmt.Errorf("error expanding path(./new-image): %v", err)
		}

		c.Infof("Restoring the new GRUB partition")
		err = c.WithCd(mount.MountPath, func(c *commands.Commands) (err error) {
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
			return nil
		})
		if err != nil {
			c.Errorf("Error restoring new GRUB partition: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		c.Errorf("Error restoring new GRUB partition: %v", err)
		return
	}
	c.Infof("Updating UUIDs in the new EFI partition")
	err = c.WithTempMountErr(efiBootDevice, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("EFI Boot device mounted at %s", mount.MountPath)
		var newUUID string
		for _, p := range newBlkIDLayout.Partitions {
			if p.Partition == updatedOriginalGRUBPartition.Partition {
				newUUID = p.UUID
			}
		}
		newGrub, err := efiBootPartition.EFIGrubTemplate.Execute(newUUID)
		if err != nil {
			c.Errorf("Error: could not generate new GRUB configuration: %v", err)
			return fmt.Errorf("error: could not generate new GRUB configuration: %v", err)
		}
		ng, err := os.Create(efiBootPartition.EFIGrubPath)
		if err != nil {
			c.Errorf("Error: could not open new GRUB configuration: %v", err)
			return fmt.Errorf("error: could not open new GRUB configuration: %v", err)
		}
		defer ng.Close()
		_, err = ng.WriteString(newGrub)
		if err != nil {
			return fmt.Errorf("error: could not write new GRUB configuration: %v", err)
		}
		return nil
	})
	if err != nil {
		c.Errorf("Error updating GRUB configuration: %v", err)
		return
	}

	c.Infof("Updating GRUB configuration on the original OS/EPIC partition")
	err = c.WithTempMountErr(originalOSPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("Original OS/EPIC device mounted at %s", mount.MountPath)
		originalMountPath := mount.MountPath
		return c.WithTempMountErr(grubDevice, "/mnt/boot", func(c *commands.Commands, mount commands.MountDetails) error {
			c.Infof("Original GRUB device mounted at %s", mount.MountPath)
			return c.WithTempMountErr(efiBootDevice, "/mnt/boot/efi", func(c *commands.Commands, mount commands.MountDetails) error {
				c.Infof("Original EFI device mounted at %s", mount.MountPath)
				c.Infof("Entering Chroot...")
				chr := c.ChrootBuilder()
				return chr.WithChrootNoMount(originalMountPath, func(proxy *commands.ChrootProxy) error {
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
					return nil
				})
			})
		})
	})
	if err != nil {
		c.Errorf("Error running grup update for the original EPIC system: %v", err)
		return
	}

	c.Infof("Updating GRUB configuration on the new OS/EPIC partition")
	err = c.WithTempMountErr(newOSPartition.Partition, "/mnt", func(c *commands.Commands, mount commands.MountDetails) error {
		c.Infof("New OS/EPIC device mounted at %s", mount.MountPath)
		originalMountPath := mount.MountPath
		return c.WithTempMountErr(newGRUBPartition.Partition, "/mnt/boot", func(c *commands.Commands, mount commands.MountDetails) error {
			c.Infof("New GRUB device mounted at %s", mount.MountPath)
			return c.WithTempMountErr(efiBootDevice, "/mnt/boot/efi", func(c *commands.Commands, mount commands.MountDetails) error {
				c.Infof("Original EFI device mounted at %s", mount.MountPath)
				c.Infof("Entering Chroot...")
				chr := c.ChrootBuilder()
				return chr.WithChrootNoMount(originalMountPath, func(proxy *commands.ChrootProxy) error {
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
					return nil
				})
			})
		})
	})
	if err != nil {
		c.Errorf("Error running grup update for the NEW EPIC system: %v", err)
		return
	}

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
