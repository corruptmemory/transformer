package commands

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

type CreateTarballStatusTag int

const writeStatusUpdateDelta = 1 * time.Second

var ErrTarballCreationTerminateByUser = errors.New("tarball creation terminated by user")

const (
	CTComputingTotalFileSize CreateTarballStatusTag = iota
	CTTotalFileSizeComputed
	CTWritingFiles
	CTSuccess
	CTError
)

type CreateTarballStatus struct {
	Tag           CreateTarballStatusTag
	SourceDir     string
	Tarball       string
	TotalFileSize uint64
	Processed     uint64
	Error         error
}

func (c *CreateTarballStatus) ProgressPercent() int {
	if c.TotalFileSize == 0 {
		return 0
	}
	return int(c.Processed * 100 / c.TotalFileSize)
}

type pipeWriteMeasurer struct {
	totalToCount uint64
	processed    uint64
	underlying   io.Writer
	callback     func(ttc uint64, processed uint64)
}

func (p *pipeWriteMeasurer) Write(b []byte) (n int, err error) {
	var sent int
	for len(b) > 0 {
		n, err = p.underlying.Write(b)
		if err != nil {
			return
		}
		b = b[n:]
		p.processed += uint64(n)
		sent += n
		p.callback(p.totalToCount, p.processed)
	}
	return sent, nil
}

func newPipeWriteMeasurer(totalToCount uint64, underlying io.Writer, callback func(ttc uint64, processed uint64)) *pipeWriteMeasurer {
	return &pipeWriteMeasurer{
		totalToCount: totalToCount,
		underlying:   underlying,
		callback:     callback,
	}
}

func (c *Commands) CreateTarball(ctx context.Context, sourceDir, tarballPath string) (statusChannel <-chan CreateTarballStatus) {
	return WithGroupInScopeReturning(c, "CreateTarball", func(c *Commands) (statusChannel <-chan CreateTarballStatus) {
		channel := make(chan CreateTarballStatus, 1000)
		statusChannel = channel
		if !strings.HasSuffix(tarballPath, ".zstd") {
			tarballPath = tarballPath + ".zstd"
		}

		computeFileSize := func() (fileSize uint64, err error) {
			select {
			case channel <- CreateTarballStatus{
				Tag:       CTComputingTotalFileSize,
				SourceDir: sourceDir,
				Tarball:   tarballPath,
			}:
			default:
				// do nothing
			}
			err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					c.Errorf("Error accessing a path %q: %v", path, err)
					return err
				}

				select {
				case <-ctx.Done():
					return ErrTarballCreationTerminateByUser
				default:
					// do nothing
				}

				if !info.IsDir() {
					fileSize += uint64(info.Size())
				}
				return nil
			})

			if err != nil {
				c.Errorf("Error walking the path: %v", err)
				return 0, err
			}
			select {
			case channel <- CreateTarballStatus{
				Tag:           CTTotalFileSizeComputed,
				SourceDir:     sourceDir,
				Tarball:       tarballPath,
				TotalFileSize: fileSize,
			}:
			default:
				// do nothing
			}
			return
		}

		buildTarball := func(totalFileSize uint64) (err error) {
			timer := time.NewTimer(writeStatusUpdateDelta)
			defer timer.Stop()
			// Open the tarball file for writing
			var tarball *os.File
			tarball, err = os.Create(tarballPath)
			if err != nil {
				c.Errorf("Error creating tarball: %v", err)
				return
			}
			defer func() {
				tarball.Close()
				if err != nil {
					os.Remove(tarballPath)
				}
			}()

			// Create a new zstd writer
			var zstdWriter *zstd.Encoder
			zstdWriter, err = zstd.NewWriter(tarball)
			if err != nil {
				c.Errorf("Error creating zstd writer: %v", err)
				return
			}
			defer zstdWriter.Close()

			pm := newPipeWriteMeasurer(totalFileSize, zstdWriter, func(ttc uint64, processed uint64) {
				select {
				case <-timer.C:
					// Doing this here to avoid blocking the tarball creation if nobody is listening to the channel
					select {
					case channel <- CreateTarballStatus{
						Tag:           CTWritingFiles,
						SourceDir:     sourceDir,
						Tarball:       tarballPath,
						TotalFileSize: totalFileSize,
						Processed:     processed,
					}:
						timer.Reset(writeStatusUpdateDelta)
					default:
						// do nothing
					}
				default:
					// do nothing
				}
			})

			tarWriter := tar.NewWriter(pm)
			defer tarWriter.Close()
			err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				select {
				case <-ctx.Done():
					return ErrTarballCreationTerminateByUser
				default:
					// do nothing
				}

				// Create a new tar header
				var header *tar.Header
				header, err = tar.FileInfoHeader(info, info.Name())
				if err != nil {
					c.Errorf("Error creating tar header: %v", err)
					return err
				}

				// Write the header to the tar archive
				err = tarWriter.WriteHeader(header)
				if err != nil {
					c.Errorf("Error writing header: %v", err)
					return err
				}

				// If it's not a directory, write the file data to the tar archive
				if !info.IsDir() {
					var file *os.File
					file, err = os.Open(path)
					if err != nil {
						c.Errorf("Error opening file: %v", err)
						return err
					}
					defer file.Close()

					_, err = io.Copy(tarWriter, file)
					if err != nil {
						c.Errorf("Error writing file data: %v", err)
						return err
					}
				}

				return nil
			})

			if err != nil {
				c.Errorf("Error building tarball: %v", err)
				return
			}

			select {
			case channel <- CreateTarballStatus{
				Tag:           CTSuccess,
				SourceDir:     sourceDir,
				Tarball:       tarballPath,
				TotalFileSize: totalFileSize,
				Processed:     pm.processed,
			}:
			default:
				// do nothing
			}
			return
		}

		master := func() {
			defer close(channel)
			fileCount, err := computeFileSize()
			if err == nil {
				err = buildTarball(fileCount)
			}
			if err != nil {
				select {
				case channel <- CreateTarballStatus{
					Tag:   CTError,
					Error: err,
				}:
				default:
					// do nothing
				}
				return
			}
		}

		go master()
		return
	})
}

type pipeReadMeasurer struct {
	totalToCount uint64
	processed    uint64
	underlying   io.Reader
	callback     func(ttc uint64, processed uint64)
}

func (p *pipeReadMeasurer) Read(b []byte) (n int, err error) {
	var read int
	for len(b) > 0 {
		n, err = p.underlying.Read(b)
		if err != nil {
			return
		}
		b = b[n:]
		p.processed += uint64(n)
		read += n
		p.callback(p.totalToCount, p.processed)
	}
	return read, nil
}

func newPipeReadMeasurer(totalToCount uint64, underlying io.Reader, callback func(ttc uint64, processed uint64)) *pipeReadMeasurer {
	return &pipeReadMeasurer{
		totalToCount: totalToCount,
		underlying:   underlying,
		callback:     callback,
	}
}

type ExtractTarballStatusTag int

var ErrTarballExtractionTerminateByUser = errors.New("tarball extraction terminated by user")

const (
	ETTarballFileSize ExtractTarballStatusTag = iota
	ETReadingFiles
	ETSuccess
	ETError
)

// 	TotalFileSize uint64
//	Processed     uint64

type ExtractTarballStatus struct {
	Tag           ExtractTarballStatusTag
	Tarball       string
	TargetDir     string
	TotalFileSize uint64
	Processed     uint64
	Error         error
}

func (e *ExtractTarballStatus) ProgressPercent() int {
	if e.TotalFileSize == 0 {
		return 0
	}
	return int(e.Processed * 100 / e.TotalFileSize)
}

func (c *Commands) ExtractTarball(ctx context.Context, tarballPath, targetDir string) (statusChannel <-chan ExtractTarballStatus) {
	return WithGroupInScopeReturning(c, "CreateTarball", func(c *Commands) (statusChannel <-chan ExtractTarballStatus) {
		channel := make(chan ExtractTarballStatus, 1000)
		statusChannel = channel

		tarballFileSize := func() (fileSize uint64, err error) {
			fi, err := os.Stat(tarballPath)
			if err != nil {
				return 0, err
			}
			fileSize = uint64(fi.Size())

			select {
			case channel <- ExtractTarballStatus{
				Tag:           ETTarballFileSize,
				TargetDir:     targetDir,
				Tarball:       tarballPath,
				TotalFileSize: fileSize,
			}:
			default:
				// do nothing
			}
			return
		}

		extractFiles := func(tarFileSize uint64) (err error) {
			timer := time.NewTimer(writeStatusUpdateDelta)
			defer timer.Stop()

			fr, err := os.Open(tarballPath)
			if err != nil {
				c.Errorf("Error opening the file: %v", err)
				return
			}
			defer fr.Close()

			rm := newPipeReadMeasurer(tarFileSize, fr, func(ttc uint64, processed uint64) {
				select {
				case <-timer.C:
					// Doing this here to avoid blocking the tarball creation if nobody is listening to the channel
					select {
					case channel <- ExtractTarballStatus{
						Tag:           ETReadingFiles,
						TargetDir:     targetDir,
						Tarball:       tarballPath,
						TotalFileSize: tarFileSize,
						Processed:     processed,
					}:
						timer.Reset(writeStatusUpdateDelta)
					default:
						// do nothing
					}
				default:
					// do nothing
				}
			})

			// Create new Zstandard reader
			zr, err := zstd.NewReader(rm)
			if err != nil {
				c.Errorf("Error creating Zstandard reader: %v", err)
				return
			}
			defer zr.Close()

			// Read the tar archive from the Zstandard reader
			tr := tar.NewReader(zr)

			for {
				header, err := tr.Next()
				if err == io.EOF {
					// End of tar archive
					break
				} else if err != nil {
					c.Errorf("Error reading tar header: %v", err)
					return err
				}

				path := filepath.Join(targetDir, header.Name)
				switch header.Typeflag {
				case tar.TypeDir:
					if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
						c.Errorf("Error creating directory %q: %v", path, err)
						return err
					}
				case tar.TypeReg:
					file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
					if err != nil {
						c.Errorf("Error creating file %q: %v", path, err)
						return err
					}
					defer file.Close()

					// Copy the file data to the new file
					if _, err := io.Copy(file, tr); err != nil {
						c.Errorf("Error copying file data: %v", err)
						return err
					}
				}
			}
			select {
			case channel <- ExtractTarballStatus{
				Tag:           ETSuccess,
				TargetDir:     targetDir,
				Tarball:       tarballPath,
				TotalFileSize: tarFileSize,
				Processed:     rm.processed,
			}:
			default:
				// do nothing
			}

			return nil
		}

		master := func() {
			defer close(channel)
			fileCount, err := tarballFileSize()
			if err == nil {
				err = extractFiles(fileCount)
			}
			if err != nil {
				select {
				case channel <- ExtractTarballStatus{
					Tag:   ETError,
					Error: err,
				}:
				default:
					// do nothing
				}
				return
			}
		}

		go master()
		return
	})
}
