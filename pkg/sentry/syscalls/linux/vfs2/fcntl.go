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
	"gvisor.dev/gvisor/pkg/syserror"
)

// Fcntl implements linux syscall fcntl(2).
func Fcntl(t *kernel.Task, args arch.SyscallArguments) (uintptr, *kernel.SyscallControl, error) {
	fd := args[0].Int()
	cmd := args[1].Int()

	file, flags := t.FDTable().GetVFS2(fd)
	if file == nil {
		return 0, nil, syserror.EBADF
	}
	defer file.DecRef()

	switch cmd {
	case linux.F_DUPFD, linux.F_DUPFD_CLOEXEC:
		from := args[2].Int()
		fd, err := t.NewFDFromVFS2(from, file, kernel.FDFlags{
			CloseOnExec: cmd == linux.F_DUPFD_CLOEXEC,
		})
		if err != nil {
			return 0, nil, err
		}
		return uintptr(fd), nil, nil
	case linux.F_GETFD:
		return uintptr(flags.ToLinuxFDFlags()), nil, nil
	case linux.F_SETFD:
		flags := args[2].Uint()
		t.FDTable().SetFlags(fd, kernel.FDFlags{
			CloseOnExec: flags&linux.FD_CLOEXEC != 0,
		})
		return 0, nil, nil
	case linux.F_GETFL:
		return uintptr(file.StatusFlags()), nil, nil
	case linux.F_SETFL:
		return 0, nil, file.SetStatusFlags(t, t.Credentials(), args[2].Uint())
	default:
		// Everything else is not yet supported.
		return 0, nil, syserror.EINVAL
	}
}
