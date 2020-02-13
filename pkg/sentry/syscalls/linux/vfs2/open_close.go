// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vfs2

import (
	"gvisor.dev/gvisor/pkg/abi/linux"
	"gvisor.dev/gvisor/pkg/sentry/arch"
	"gvisor.dev/gvisor/pkg/sentry/kernel"
	slinux "gvisor.dev/gvisor/pkg/sentry/syscalls/linux"
	"gvisor.dev/gvisor/pkg/sentry/vfs"
	"gvisor.dev/gvisor/pkg/syserror"
	"gvisor.dev/gvisor/pkg/usermem"
)

// Open implements Linux syscall open(2).
func Open(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	addr := args[0].Pointer()
	flags := args[1].Uint()
	mode := linux.FileMode(args[2].ModeT())
	return openat(t, linux.AT_FDCWD, addr, flags, mode)
}

// Openat implements Linux syscall openat(2).
func Openat(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	dirfd := args[0].Int()
	addr := args[1].Pointer()
	flags := args[2].Uint()
	mode := linux.FileMode(args[3].ModeT())
	return openat(t, dirfd, addr, flags, mode)
}

// Creat implements Linux syscall creat(2).
func Creat(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	addr := args[0].Pointer()
	mode := linux.FileMode(args[1].ModeT())
	return openat(t, linux.AT_FDCWD, addr, linux.O_WRONLY|linux.O_CREAT|linux.O_TRUNC, mode)
}

func openat(t *kernel.Task, dirfd int32, pathAddr usermem.Addr, flags uint32, mode linux.FileMode) (uintptr, *kernel.SyscallControl, error) {
	path, err := copyInPath(t, pathAddr)
	if err != nil {
		return 0, nil, err
	}
	tpop, err := getTaskPathOperation(t, dirfd, path, disallowEmptyPath, shouldFollowFinalSymlink(flags&linux.O_NOFOLLOW == 0))
	if err != nil {
		return 0, nil, err
	}
	defer tpop.Release()

	file, err := t.Kernel().VFS().OpenAt(t, t.Credentials(), &tpop.pop, &vfs.OpenOptions{
		Flags: flags,
		Mode:  mode &^ linux.FileMode(t.FSContext().Umask()),
	})
	if err != nil {
		return 0, nil, err
	}
	defer file.DecRef()

	fd, err := t.NewFDFromVFS2(0, file, kernel.FDFlags{
		CloseOnExec: flags&linux.O_CLOEXEC != 0,
	})
	return uintptr(fd), nil, err
}

// Close implements Linux syscall close(2).
func Close(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	fd := args[0].Int()

	// Note that Remove provides a reference on the file that we may use to
	// flush. It is still active until we drop the final reference below
	// (and other reference-holding operations complete).
	_, file := t.FDTable().Remove(fd)
	if file == nil {
		return 0, nil, syserror.EBADF
	}
	defer file.DecRef()

	err := file.OnClose(t)
	return 0, nil, slinux.HandleIOErrorVFS2(t, false /* partial */, err, syserror.EINTR, "close", file)
}

// Dup implements Linux syscall dup(2).
func Dup(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	fd := args[0].Int()

	file := t.GetFileVFS2(fd)
	if file == nil {
		return 0, nil, syserror.EBADF
	}
	defer file.DecRef()

	newFD, err := t.NewFDFromVFS2(0, file, kernel.FDFlags{})
	if err != nil {
		return 0, nil, syserror.EMFILE
	}
	return uintptr(newFD), nil, nil
}

// Dup2 implements Linux syscall dup2(2).
func Dup2(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	oldfd := args[0].Int()
	newfd := args[1].Int()

	if oldfd == newfd {
		// As long as oldfd is valid, dup2() does nothing and returns newfd.
		file := t.GetFileVFS2(oldfd)
		if file == nil {
			return 0, nil, syserror.EBADF
		}
		file.DecRef()
		return uintptr(newfd), nil, nil
	}

	return dup3(t, oldfd, newfd, 0)
}

// Dup3 implements Linux syscall dup3(2).
func Dup3(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	oldfd := args[0].Int()
	newfd := args[1].Int()
	flags := args[2].Uint()

	if oldfd == newfd {
		return 0, nil, syserror.EINVAL
	}

	return dup3(t, oldfd, newfd, flags)
}

func dup3(t *kernel.Task, oldfd, newfd int32, flags uint32) (uintptr, *kernel.SyscallControl, error) {
	if flags&^linux.O_CLOEXEC != 0 {
		return 0, nil, syserror.EINVAL
	}

	file := t.GetFileVFS2(oldfd)
	if file == nil {
		return 0, nil, syserror.EBADF
	}
	defer file.DecRef()

	err := t.NewFDAtVFS2(newfd, file, kernel.FDFlags{
		CloseOnExec: flags&linux.O_CLOEXEC != 0,
	})
	if err != nil {
		return 0, nil, err
	}
	return uintptr(newfd), nil, nil
}
