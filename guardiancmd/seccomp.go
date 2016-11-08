package guardiancmd

import "github.com/opencontainers/runtime-spec/specs-go"

var seccomp = &specs.LinuxSeccomp{
	DefaultAction: specs.ActErrno,
	Architectures: []specs.Arch{
		specs.ArchX86_64,
		specs.ArchX86,
		specs.ArchX32,
	},
	Syscalls: []specs.LinuxSyscall{
		specs.LinuxSyscall{
			Name:   "accept",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "accept4",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "access",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "alarm",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "bind",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "brk",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "capget",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "capset",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "chdir",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "chmod",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "chown",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "chown32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "clock_getres",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "clock_gettime",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "clock_nanosleep",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "close",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "connect",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "copy_file_range",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "creat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "dup",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "dup2",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "dup3",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "epoll_create",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "epoll_create1",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "epoll_ctl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "epoll_ctl_old",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "epoll_pwait",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "epoll_wait",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "epoll_wait_old",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "eventfd",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "eventfd2",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "execve",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "execveat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "exit",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "exit_group",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "faccessat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fadvise64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fadvise64_64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fallocate",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fanotify_mark",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fchdir",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fchmod",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fchmodat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fchown",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fchown32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fchownat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fcntl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fcntl64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fdatasync",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fgetxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "flistxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "flock",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fork",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fremovexattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fsetxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fstat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fstat64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fstatat64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fstatfs",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fstatfs64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "fsync",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ftruncate",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ftruncate64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "futex",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "futimesat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getcpu",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getcwd",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getdents",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getdents64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getegid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getegid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "geteuid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "geteuid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getgid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getgid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getgroups",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getgroups32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getitimer",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getpeername",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getpgid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getpgrp",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getpid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getppid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getpriority",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getrandom",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getresgid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getresgid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getresuid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getresuid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getrlimit",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "get_robust_list",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getrusage",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getsid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getsockname",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getsockopt",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "get_thread_area",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "gettid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "gettimeofday",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getuid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getuid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "getxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "inotify_add_watch",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "inotify_init",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "inotify_init1",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "inotify_rm_watch",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "io_cancel",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ioctl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "io_destroy",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "io_getevents",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ioprio_get",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ioprio_set",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "io_setup",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "io_submit",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ipc",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "kill",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lchown",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lchown32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lgetxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "link",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "linkat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "listen",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "listxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "llistxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "_llseek",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lremovexattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lseek",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lsetxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lstat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "lstat64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "madvise",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "memfd_create",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mincore",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mkdir",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mkdirat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mknod",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mknodat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mlock",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mlock2",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mlockall",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mmap",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mmap2",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mprotect",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mq_getsetattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mq_notify",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mq_open",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mq_timedreceive",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mq_timedsend",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mq_unlink",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "mremap",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "msgctl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "msgget",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "msgrcv",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "msgsnd",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "msync",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "munlock",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "munlockall",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "munmap",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "nanosleep",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "newfstatat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "_newselect",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "open",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "openat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "pause",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "personality",
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    0,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			},
		},
		specs.LinuxSyscall{
			Name:   "personality",
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    8,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			},
		},
		specs.LinuxSyscall{
			Name:   "personality",
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    4294967295,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			},
		},
		specs.LinuxSyscall{
			Name:   "pipe",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "pipe2",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "poll",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ppoll",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "prctl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "pread64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "preadv",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "prlimit64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "pselect6",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "pwrite64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "pwritev",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "read",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "readahead",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "readlink",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "readlinkat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "readv",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "recv",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "recvfrom",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "recvmmsg",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "recvmsg",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "remap_file_pages",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "removexattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rename",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "renameat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "renameat2",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "restart_syscall",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rmdir",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_sigaction",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_sigpending",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_sigprocmask",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_sigqueueinfo",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_sigreturn",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_sigsuspend",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_sigtimedwait",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "rt_tgsigqueueinfo",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_getaffinity",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_getattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_getparam",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_get_priority_max",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_get_priority_min",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_getscheduler",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_rr_get_interval",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_setaffinity",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_setattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_setparam",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_setscheduler",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sched_yield",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "seccomp",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "select",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "semctl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "semget",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "semop",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "semtimedop",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "send",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sendfile",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sendfile64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sendmmsg",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sendmsg",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sendto",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setfsgid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setfsgid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setfsuid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setfsuid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setgid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setgid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setgroups",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setgroups32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setitimer",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setpgid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setpriority",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setregid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setregid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setresgid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setresgid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setresuid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setresuid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setreuid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setreuid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setrlimit",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "set_robust_list",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setsid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setsockopt",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "set_thread_area",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "set_tid_address",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setuid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setuid32",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "setxattr",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "shmat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "shmctl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "shmdt",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "shmget",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "shutdown",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sigaltstack",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "signalfd",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "signalfd4",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sigreturn",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "socket",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "socketcall",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "socketpair",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "splice",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "stat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "stat64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "statfs",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "statfs64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "symlink",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "symlinkat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sync",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sync_file_range",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "syncfs",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "sysinfo",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "syslog",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "tee",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "tgkill",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "time",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timer_create",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timer_delete",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timerfd_create",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timerfd_gettime",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timerfd_settime",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timer_getoverrun",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timer_gettime",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "timer_settime",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "times",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "tkill",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "truncate",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "truncate64",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "ugetrlimit",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "umask",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "uname",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "unlink",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "unlinkat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "utime",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "utimensat",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "utimes",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "vfork",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "vmsplice",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "wait4",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "waitid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "waitpid",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "write",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "writev",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "arch_prctl",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "modify_ldt",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "chroot",
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Name:   "clone",
			Action: specs.ActAllow,
			Args: []specs.LinuxSeccompArg{
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    2080505856,
					ValueTwo: 0,
					Op:       specs.OpMaskedEqual,
				},
			},
		},
	},
}
