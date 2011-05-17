// Copyright (c) 2011 The Grumble Authors
// The use of this source code is goverened by a BSD-style
// license that can be found in the LICENSE-file.

package blobstore

import (
	"os"
	"path/filepath"
	"io/ioutil"
	"strconv"
	"syscall"
)

// Acquire lockfile at path.
func AcquireLockFile(path string) os.Error {
	dir, fn := filepath.Split(path)
	lockfn := filepath.Join(dir, fn)

	lockfile, err := os.OpenFile(lockfn, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e, ok := err.(*os.PathError); ok && e.Error == os.EEXIST {
		content, err := ioutil.ReadFile(lockfn)
		if err != nil {
			return err
		}

		pid, err := strconv.Atoi(string(content))
		if err == nil {
			if syscall.Kill(pid, 0) == 0 {
				return ErrLocked
			}
		}

		lockfile, err = ioutil.TempFile(dir, "lock")
		if err != nil {
			return err
		}

		_, err = lockfile.WriteString(strconv.Itoa(syscall.Getpid()))
		if err != nil {
			lockfile.Close()
			return ErrLockAcquirement
		}

		curfn := lockfile.Name()

		err = lockfile.Close()
		if err != nil {
			return err
		}

		err = os.Rename(curfn, lockfn)
		if err != nil {
			os.Remove(curfn)
			return ErrLockAcquirement
		}
	} else if err != nil {
		return err
	} else {
		_, err = lockfile.WriteString(strconv.Itoa(syscall.Getpid()))
		if err != nil {
			return err
		}
		lockfile.Close()
	}

	return nil
}

// Release lockfile at path.
func ReleaseLockFile(path string) os.Error {
	return os.Remove(path)
}