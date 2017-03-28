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
			Names:  []string{"accept"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"accept4"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"access"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"alarm"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"bind"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"brk"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"capget"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"capset"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"chdir"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"chmod"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"chown"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"chown32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"clock_getres"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"clock_gettime"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"clock_nanosleep"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"close"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"connect"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"copy_file_range"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"creat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"dup"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"dup2"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"dup3"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"epoll_create"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"epoll_create1"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"epoll_ctl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"epoll_ctl_old"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"epoll_pwait"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"epoll_wait"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"epoll_wait_old"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"eventfd"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"eventfd2"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"execve"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"execveat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"exit"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"exit_group"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"faccessat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fadvise64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fadvise64_64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fallocate"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fanotify_mark"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fchdir"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fchmod"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fchmodat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fchown"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fchown32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fchownat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fcntl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fcntl64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fdatasync"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fgetxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"flistxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"flock"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fork"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fremovexattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fsetxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fstat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fstat64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fstatat64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fstatfs"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fstatfs64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"fsync"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ftruncate"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ftruncate64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"futex"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"futimesat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getcpu"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getcwd"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getdents"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getdents64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getegid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getegid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"geteuid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"geteuid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getgid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getgid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getgroups"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getgroups32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getitimer"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getpeername"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getpgid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getpgrp"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getpid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getppid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getpriority"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getrandom"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getresgid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getresgid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getresuid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getresuid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getrlimit"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"get_robust_list"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getrusage"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getsid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getsockname"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getsockopt"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"get_thread_area"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"gettid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"gettimeofday"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getuid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getuid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"getxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"inotify_add_watch"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"inotify_init"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"inotify_init1"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"inotify_rm_watch"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"io_cancel"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ioctl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"io_destroy"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"io_getevents"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ioprio_get"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ioprio_set"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"io_setup"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"io_submit"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ipc"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"kill"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lchown"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lchown32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lgetxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"link"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"linkat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"listen"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"listxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"llistxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"_llseek"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lremovexattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lseek"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lsetxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lstat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"lstat64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"madvise"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"memfd_create"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mincore"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mkdir"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mkdirat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mknod"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mknodat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mlock"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mlock2"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mlockall"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mmap"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mmap2"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mprotect"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mq_getsetattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mq_notify"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mq_open"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mq_timedreceive"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mq_timedsend"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mq_unlink"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"mremap"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"msgctl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"msgget"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"msgrcv"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"msgsnd"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"msync"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"munlock"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"munlockall"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"munmap"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"nanosleep"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"newfstatat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"_newselect"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"open"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"openat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"pause"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"personality"},
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
			Names:  []string{"personality"},
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
			Names:  []string{"personality"},
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
			Names:  []string{"pipe"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"pipe2"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"poll"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ppoll"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"prctl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"pread64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"preadv"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"prlimit64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"pselect6"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"pwrite64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"pwritev"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"read"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"readahead"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"readlink"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"readlinkat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"readv"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"recv"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"recvfrom"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"recvmmsg"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"recvmsg"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"remap_file_pages"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"removexattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rename"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"renameat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"renameat2"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"restart_syscall"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rmdir"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_sigaction"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_sigpending"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_sigprocmask"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_sigqueueinfo"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_sigreturn"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_sigsuspend"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_sigtimedwait"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"rt_tgsigqueueinfo"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_getaffinity"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_getattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_getparam"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_get_priority_max"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_get_priority_min"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_getscheduler"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_rr_get_interval"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_setaffinity"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_setattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_setparam"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_setscheduler"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sched_yield"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"seccomp"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"select"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"semctl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"semget"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"semop"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"semtimedop"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"send"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sendfile"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sendfile64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sendmmsg"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sendmsg"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sendto"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setfsgid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setfsgid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setfsuid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setfsuid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setgid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setgid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setgroups"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setgroups32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setitimer"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setpgid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setpriority"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setregid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setregid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setresgid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setresgid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setresuid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setresuid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setreuid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setreuid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setrlimit"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"set_robust_list"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setsid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setsockopt"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"set_thread_area"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"set_tid_address"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setuid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setuid32"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"setxattr"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"shmat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"shmctl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"shmdt"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"shmget"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"shutdown"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sigaltstack"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"signalfd"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"signalfd4"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sigreturn"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"socket"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"socketcall"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"socketpair"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"splice"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"stat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"stat64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"statfs"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"statfs64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"symlink"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"symlinkat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sync"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sync_file_range"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"syncfs"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"sysinfo"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"syslog"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"tee"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"tgkill"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"time"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timer_create"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timer_delete"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timerfd_create"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timerfd_gettime"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timerfd_settime"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timer_getoverrun"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timer_gettime"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"timer_settime"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"times"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"tkill"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"truncate"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"truncate64"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"ugetrlimit"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"umask"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"uname"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"unlink"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"unlinkat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"utime"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"utimensat"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"utimes"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"vfork"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"vmsplice"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"wait4"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"waitid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"waitpid"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"write"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"writev"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"arch_prctl"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"modify_ldt"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"chroot"},
			Action: specs.ActAllow,
			Args:   []specs.LinuxSeccompArg{},
		},
		specs.LinuxSyscall{
			Names:  []string{"clone"},
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
