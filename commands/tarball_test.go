package commands

import (
	"context"
	"testing"
)

func Test_CreateTarball(t *testing.T) {
	t.Skipf("Skipping this test because it only runs on my machine")
	cmds, err := DefaultCommands(false)
	if err != nil {
		t.Fatalf("error creating commands: %v", err)
	}
	ctx := context.Background()
	events := cmds.CreateTarball(ctx, "/home/jim/saved-downloads", "/home/jim/saved-downloads.tar.zstd")
	for event := range events {
		switch event.Tag {
		case CTComputingTotalFileSize:
			t.Logf("counting files: %s", event.SourceDir)
		case CTTotalFileSizeComputed:
			t.Logf("count finished: %s -- %d bytes", event.SourceDir, event.TotalFileSize)
		case CTWritingFiles:
			t.Logf("writing files[%d%%]: %s -> %s", event.ProgressPercent(), event.SourceDir, event.Tarball)
		case CTSuccess:
			t.Logf("success[%d%%]: %s -> %s", event.ProgressPercent(), event.SourceDir, event.Tarball)
		case CTError:
			t.Logf("error: %s -> %s: %v", event.SourceDir, event.Tarball, event.Error)
			t.Errorf("error: %s -> %s: %v", event.SourceDir, event.Tarball, event.Error)
		}
	}
	eevents := cmds.ExtractTarball(ctx, "/home/jim/saved-downloads.tar.zstd", "/home/jim/saved-downloads-extracted")
	for event := range eevents {
		switch event.Tag {
		case ETTarballFileSize:
			t.Logf("tarball file size: %s -- %d bytes", event.Tarball, event.TotalFileSize)
		case ETReadingFiles:
			t.Logf("reading files[%d%%]: %s -> %s", event.ProgressPercent(), event.Tarball, event.TargetDir)
		case ETSuccess:
			t.Logf("success[%d%%]: %s -> %s", event.ProgressPercent(), event.Tarball, event.TargetDir)
		case ETError:
			t.Logf("error: %s -> %s: %v", event.Tarball, event.TargetDir, event.Error)
			t.Errorf("error: %s -> %s: %v", event.Tarball, event.TargetDir, event.Error)
		}
	}
}
