// Copyright (c) 2025-2026, s0up4200 <s0up4200@pm.me> and the mkbrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package torrent

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
)

var publishTorrentOutput = publishTorrentFile

func writeTorrentOutput(outputPath string, newPerm os.FileMode, replace bool, write func(io.Writer) error) error {
	perm := newPerm
	old, err := os.Lstat(outputPath)
	replacing := replace && err == nil && old.Mode().IsRegular()
	if replacing {
		err := retryWhileShared(func() error {
			probe, err := os.OpenFile(outputPath, os.O_RDWR, 0)
			if err == nil {
				_ = probe.Close()
			}
			return err
		})
		if err != nil {
			return fmt.Errorf("error creating output file: %w", err)
		}
		perm = old.Mode().Perm()
	}
	f, unregister, err := createTempTorrentFile(outputPath, perm)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer unregister()
	if replacing {
		err = f.Chmod(perm)
	}
	if err == nil {
		err = write(f)
	}
	if err == nil {
		err = f.Sync()
	}
	if err = cmp.Or(err, f.Close()); err == nil {
		err = publishTorrentOutput(f.Name(), outputPath, replace)
	}
	if err != nil {
		return errors.Join(fmt.Errorf("error writing torrent file %q: %w", outputPath, err), os.Remove(f.Name()))
	}
	return nil
}
