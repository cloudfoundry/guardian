package guardiancmd

import "github.com/opencontainers/runtime-spec/specs-go"

var seccomp = &specs.Seccomp{
	DefaultAction: specs.ActErrno,
	Architectures: []specs.Arch{
		specs.ArchX86_64,
		specs.ArchX86,
		specs.ArchX32,
	},
	Syscalls: []specs.Syscall{
		specs.Syscall{
			Name:   "accept",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "accept4",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "access",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "alarm",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "bind",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "brk",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "capget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "capset",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "chdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "chmod",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "chown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "chown32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "clock_getres",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "clock_gettime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "clock_nanosleep",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "close",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "connect",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "copy_file_range",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "creat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "dup",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "dup2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "dup3",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "epoll_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "epoll_create1",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "epoll_ctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "epoll_ctl_old",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "epoll_pwait",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "epoll_wait",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "epoll_wait_old",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "eventfd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "eventfd2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "execve",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "execveat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "exit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "exit_group",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "faccessat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fadvise64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fadvise64_64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fallocate",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fanotify_mark",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fchdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fchmod",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fchmodat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fchown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fchown32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fchownat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fcntl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fcntl64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fdatasync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fgetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "flistxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "flock",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fork",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fremovexattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fsetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fstat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fstat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fstatat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fstatfs",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fstatfs64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "fsync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ftruncate",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ftruncate64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "futex",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "futimesat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getcpu",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getcwd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getdents",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getdents64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getegid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getegid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "geteuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "geteuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getgroups",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getgroups32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getitimer",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getpeername",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getpgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getpgrp",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getpid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getppid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getpriority",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getrandom",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getresgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getresgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getresuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getresuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getrlimit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "get_robust_list",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getrusage",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getsid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getsockname",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getsockopt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "get_thread_area",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "gettid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "gettimeofday",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "getxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "inotify_add_watch",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "inotify_init",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "inotify_init1",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "inotify_rm_watch",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "io_cancel",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ioctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "io_destroy",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "io_getevents",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ioprio_get",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ioprio_set",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "io_setup",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "io_submit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ipc",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "kill",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lchown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lchown32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lgetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "link",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "linkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "listen",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "listxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "llistxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "_llseek",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lremovexattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lseek",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lsetxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lstat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "lstat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "madvise",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "memfd_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mincore",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mkdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mkdirat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mknod",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mknodat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mlock",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mlock2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mlockall",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mmap",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mmap2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mprotect",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mq_getsetattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mq_notify",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mq_open",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mq_timedreceive",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mq_timedsend",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mq_unlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "mremap",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "msgctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "msgget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "msgrcv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "msgsnd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "msync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "munlock",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "munlockall",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "munmap",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "nanosleep",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "newfstatat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "_newselect",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "open",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "openat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "pause",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "personality",
			Action: specs.ActAllow,
			Args: []specs.Arg{
				specs.Arg{
					Index:    0,
					Value:    0,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			},
		},
		specs.Syscall{
			Name:   "personality",
			Action: specs.ActAllow,
			Args: []specs.Arg{
				specs.Arg{
					Index:    0,
					Value:    8,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			},
		},
		specs.Syscall{
			Name:   "personality",
			Action: specs.ActAllow,
			Args: []specs.Arg{
				specs.Arg{
					Index:    0,
					Value:    4294967295,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			},
		},
		specs.Syscall{
			Name:   "pipe",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "pipe2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "poll",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ppoll",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "prctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "pread64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "preadv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "prlimit64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "pselect6",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "pwrite64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "pwritev",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "read",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "readahead",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "readlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "readlinkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "readv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "recv",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "recvfrom",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "recvmmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "recvmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "remap_file_pages",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "removexattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rename",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "renameat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "renameat2",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "restart_syscall",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rmdir",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_sigaction",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_sigpending",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_sigprocmask",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_sigqueueinfo",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_sigreturn",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_sigsuspend",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_sigtimedwait",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "rt_tgsigqueueinfo",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_getaffinity",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_getattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_getparam",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_get_priority_max",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_get_priority_min",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_getscheduler",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_rr_get_interval",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_setaffinity",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_setattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_setparam",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_setscheduler",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sched_yield",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "seccomp",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "select",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "semctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "semget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "semop",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "semtimedop",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "send",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sendfile",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sendfile64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sendmmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sendmsg",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sendto",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setfsgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setfsgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setfsuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setfsuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setgroups",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setgroups32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setitimer",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setpgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setpriority",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setregid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setregid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setresgid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setresgid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setresuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setresuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setreuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setreuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setrlimit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "set_robust_list",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setsid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setsockopt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "set_thread_area",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "set_tid_address",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setuid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setuid32",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "setxattr",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "shmat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "shmctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "shmdt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "shmget",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "shutdown",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sigaltstack",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "signalfd",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "signalfd4",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sigreturn",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "socket",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "socketcall",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "socketpair",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "splice",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "stat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "stat64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "statfs",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "statfs64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "symlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "symlinkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sync",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sync_file_range",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "syncfs",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "sysinfo",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "syslog",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "tee",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "tgkill",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "time",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timer_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timer_delete",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timerfd_create",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timerfd_gettime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timerfd_settime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timer_getoverrun",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timer_gettime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "timer_settime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "times",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "tkill",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "truncate",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "truncate64",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "ugetrlimit",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "umask",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "uname",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "unlink",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "unlinkat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "utime",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "utimensat",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "utimes",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "vfork",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "vmsplice",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "wait4",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "waitid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "waitpid",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "write",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "writev",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "arch_prctl",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "modify_ldt",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "chroot",
			Action: specs.ActAllow,
			Args:   []specs.Arg{},
		},
		specs.Syscall{
			Name:   "clone",
			Action: specs.ActAllow,
			Args: []specs.Arg{
				specs.Arg{
					Index:    0,
					Value:    2080505856,
					ValueTwo: 0,
					Op:       specs.OpMaskedEqual,
				},
			},
		},
	},
}
