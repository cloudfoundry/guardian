package guardiancmd

import (
	"code.cloudfoundry.org/guardian/rundmc/sysctl"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func MinPtraceKernelVersion() (uint16, uint16, uint16) {
	return 4, 8, 0
}

func buildSeccomp() (*specs.LinuxSeccomp, error) {
	seccomp := &specs.LinuxSeccomp{
		DefaultAction: specs.ActErrno,
		Architectures: []specs.Arch{
			specs.ArchX86_64,
			specs.ArchX86,
			specs.ArchX32,
		},
		Syscalls: []specs.LinuxSyscall{
			AllowSyscall("accept"),
			AllowSyscall("accept4"),
			AllowSyscall("access"),
			AllowSyscall("alarm"),
			AllowSyscall("bind"),
			AllowSyscall("brk"),
			AllowSyscall("capget"),
			AllowSyscall("capset"),
			AllowSyscall("chdir"),
			AllowSyscall("chmod"),
			AllowSyscall("chown"),
			AllowSyscall("chown32"),
			AllowSyscall("clock_getres"),
			AllowSyscall("clock_gettime"),
			AllowSyscall("clock_nanosleep"),
			AllowSyscall("close"),
			AllowSyscall("connect"),
			AllowSyscall("copy_file_range"),
			AllowSyscall("creat"),
			AllowSyscall("dup"),
			AllowSyscall("dup2"),
			AllowSyscall("dup3"),
			AllowSyscall("epoll_create"),
			AllowSyscall("epoll_create1"),
			AllowSyscall("epoll_ctl"),
			AllowSyscall("epoll_ctl_old"),
			AllowSyscall("epoll_pwait"),
			AllowSyscall("epoll_wait"),
			AllowSyscall("epoll_wait_old"),
			AllowSyscall("eventfd"),
			AllowSyscall("eventfd2"),
			AllowSyscall("execve"),
			AllowSyscall("execveat"),
			AllowSyscall("exit"),
			AllowSyscall("exit_group"),
			AllowSyscall("faccessat"),
			AllowSyscall("fadvise64"),
			AllowSyscall("fadvise64_64"),
			AllowSyscall("fallocate"),
			AllowSyscall("fanotify_mark"),
			AllowSyscall("fchdir"),
			AllowSyscall("fchmod"),
			AllowSyscall("fchmodat"),
			AllowSyscall("fchown"),
			AllowSyscall("fchown32"),
			AllowSyscall("fchownat"),
			AllowSyscall("fcntl"),
			AllowSyscall("fcntl64"),
			AllowSyscall("fdatasync"),
			AllowSyscall("fgetxattr"),
			AllowSyscall("flistxattr"),
			AllowSyscall("flock"),
			AllowSyscall("fork"),
			AllowSyscall("fremovexattr"),
			AllowSyscall("fsetxattr"),
			AllowSyscall("fstat"),
			AllowSyscall("fstat64"),
			AllowSyscall("fstatat64"),
			AllowSyscall("fstatfs"),
			AllowSyscall("fstatfs64"),
			AllowSyscall("fsync"),
			AllowSyscall("ftruncate"),
			AllowSyscall("ftruncate64"),
			AllowSyscall("futex"),
			AllowSyscall("futimesat"),
			AllowSyscall("getcpu"),
			AllowSyscall("getcwd"),
			AllowSyscall("getdents"),
			AllowSyscall("getdents64"),
			AllowSyscall("getegid"),
			AllowSyscall("getegid32"),
			AllowSyscall("geteuid"),
			AllowSyscall("geteuid32"),
			AllowSyscall("getgid"),
			AllowSyscall("getgid32"),
			AllowSyscall("getgroups"),
			AllowSyscall("getgroups32"),
			AllowSyscall("getitimer"),
			AllowSyscall("getpeername"),
			AllowSyscall("getpgid"),
			AllowSyscall("getpgrp"),
			AllowSyscall("getpid"),
			AllowSyscall("getppid"),
			AllowSyscall("getpriority"),
			AllowSyscall("getrandom"),
			AllowSyscall("getresgid"),
			AllowSyscall("getresgid32"),
			AllowSyscall("getresuid"),
			AllowSyscall("getresuid32"),
			AllowSyscall("getrlimit"),
			AllowSyscall("get_robust_list"),
			AllowSyscall("getrusage"),
			AllowSyscall("getsid"),
			AllowSyscall("getsockname"),
			AllowSyscall("getsockopt"),
			AllowSyscall("get_thread_area"),
			AllowSyscall("gettid"),
			AllowSyscall("gettimeofday"),
			AllowSyscall("getuid"),
			AllowSyscall("getuid32"),
			AllowSyscall("getxattr"),
			AllowSyscall("inotify_add_watch"),
			AllowSyscall("inotify_init"),
			AllowSyscall("inotify_init1"),
			AllowSyscall("inotify_rm_watch"),
			AllowSyscall("io_cancel"),
			AllowSyscall("ioctl"),
			AllowSyscall("io_destroy"),
			AllowSyscall("io_getevents"),
			AllowSyscall("ioprio_get"),
			AllowSyscall("ioprio_set"),
			AllowSyscall("io_setup"),
			AllowSyscall("io_submit"),
			AllowSyscall("ipc"),
			AllowSyscall("kill"),
			AllowSyscall("lchown"),
			AllowSyscall("lchown32"),
			AllowSyscall("lgetxattr"),
			AllowSyscall("link"),
			AllowSyscall("linkat"),
			AllowSyscall("listen"),
			AllowSyscall("listxattr"),
			AllowSyscall("llistxattr"),
			AllowSyscall("_llseek"),
			AllowSyscall("lremovexattr"),
			AllowSyscall("lseek"),
			AllowSyscall("lsetxattr"),
			AllowSyscall("lstat"),
			AllowSyscall("lstat64"),
			AllowSyscall("madvise"),
			AllowSyscall("memfd_create"),
			AllowSyscall("mincore"),
			AllowSyscall("mkdir"),
			AllowSyscall("mkdirat"),
			AllowSyscall("mknod"),
			AllowSyscall("mknodat"),
			AllowSyscall("mlock"),
			AllowSyscall("mlock2"),
			AllowSyscall("mlockall"),
			AllowSyscall("mmap"),
			AllowSyscall("mmap2"),
			AllowSyscall("mprotect"),
			AllowSyscall("mq_getsetattr"),
			AllowSyscall("mq_notify"),
			AllowSyscall("mq_open"),
			AllowSyscall("mq_timedreceive"),
			AllowSyscall("mq_timedsend"),
			AllowSyscall("mq_unlink"),
			AllowSyscall("mremap"),
			AllowSyscall("msgctl"),
			AllowSyscall("msgget"),
			AllowSyscall("msgrcv"),
			AllowSyscall("msgsnd"),
			AllowSyscall("msync"),
			AllowSyscall("munlock"),
			AllowSyscall("munlockall"),
			AllowSyscall("munmap"),
			AllowSyscall("nanosleep"),
			AllowSyscall("newfstatat"),
			AllowSyscall("_newselect"),
			AllowSyscall("open"),
			AllowSyscall("openat"),
			AllowSyscall("pause"),
			AllowSyscall("personality",
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    0,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			),
			AllowSyscall("personality",
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    8,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			),
			AllowSyscall("personality",
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    4294967295,
					ValueTwo: 0,
					Op:       specs.OpEqualTo,
				},
			),
			AllowSyscall("pipe"),
			AllowSyscall("pipe2"),
			AllowSyscall("poll"),
			AllowSyscall("ppoll"),
			AllowSyscall("prctl"),
			AllowSyscall("pread64"),
			AllowSyscall("preadv"),
			AllowSyscall("prlimit64"),
			AllowSyscall("pselect6"),
			AllowSyscall("pwrite64"),
			AllowSyscall("pwritev"),
			AllowSyscall("read"),
			AllowSyscall("readahead"),
			AllowSyscall("readlink"),
			AllowSyscall("readlinkat"),
			AllowSyscall("readv"),
			AllowSyscall("recv"),
			AllowSyscall("recvfrom"),
			AllowSyscall("recvmmsg"),
			AllowSyscall("recvmsg"),
			AllowSyscall("remap_file_pages"),
			AllowSyscall("removexattr"),
			AllowSyscall("rename"),
			AllowSyscall("renameat"),
			AllowSyscall("renameat2"),
			AllowSyscall("restart_syscall"),
			AllowSyscall("rmdir"),
			AllowSyscall("rt_sigaction"),
			AllowSyscall("rt_sigpending"),
			AllowSyscall("rt_sigprocmask"),
			AllowSyscall("rt_sigqueueinfo"),
			AllowSyscall("rt_sigreturn"),
			AllowSyscall("rt_sigsuspend"),
			AllowSyscall("rt_sigtimedwait"),
			AllowSyscall("rt_tgsigqueueinfo"),
			AllowSyscall("sched_getaffinity"),
			AllowSyscall("sched_getattr"),
			AllowSyscall("sched_getparam"),
			AllowSyscall("sched_get_priority_max"),
			AllowSyscall("sched_get_priority_min"),
			AllowSyscall("sched_getscheduler"),
			AllowSyscall("sched_rr_get_interval"),
			AllowSyscall("sched_setaffinity"),
			AllowSyscall("sched_setattr"),
			AllowSyscall("sched_setparam"),
			AllowSyscall("sched_setscheduler"),
			AllowSyscall("sched_yield"),
			AllowSyscall("seccomp"),
			AllowSyscall("select"),
			AllowSyscall("semctl"),
			AllowSyscall("semget"),
			AllowSyscall("semop"),
			AllowSyscall("semtimedop"),
			AllowSyscall("send"),
			AllowSyscall("sendfile"),
			AllowSyscall("sendfile64"),
			AllowSyscall("sendmmsg"),
			AllowSyscall("sendmsg"),
			AllowSyscall("sendto"),
			AllowSyscall("setfsgid"),
			AllowSyscall("setfsgid32"),
			AllowSyscall("setfsuid"),
			AllowSyscall("setfsuid32"),
			AllowSyscall("setgid"),
			AllowSyscall("setgid32"),
			AllowSyscall("setgroups"),
			AllowSyscall("setgroups32"),
			AllowSyscall("setitimer"),
			AllowSyscall("setpgid"),
			AllowSyscall("setpriority"),
			AllowSyscall("setregid"),
			AllowSyscall("setregid32"),
			AllowSyscall("setresgid"),
			AllowSyscall("setresgid32"),
			AllowSyscall("setresuid"),
			AllowSyscall("setresuid32"),
			AllowSyscall("setreuid"),
			AllowSyscall("setreuid32"),
			AllowSyscall("setrlimit"),
			AllowSyscall("set_robust_list"),
			AllowSyscall("setsid"),
			AllowSyscall("setsockopt"),
			AllowSyscall("set_thread_area"),
			AllowSyscall("set_tid_address"),
			AllowSyscall("setuid"),
			AllowSyscall("setuid32"),
			AllowSyscall("setxattr"),
			AllowSyscall("shmat"),
			AllowSyscall("shmctl"),
			AllowSyscall("shmdt"),
			AllowSyscall("shmget"),
			AllowSyscall("shutdown"),
			AllowSyscall("sigaltstack"),
			AllowSyscall("signalfd"),
			AllowSyscall("signalfd4"),
			AllowSyscall("sigreturn"),
			AllowSyscall("socket"),
			AllowSyscall("socketcall"),
			AllowSyscall("socketpair"),
			AllowSyscall("splice"),
			AllowSyscall("stat"),
			AllowSyscall("stat64"),
			AllowSyscall("statx"),
			AllowSyscall("statfs"),
			AllowSyscall("statfs64"),
			AllowSyscall("symlink"),
			AllowSyscall("symlinkat"),
			AllowSyscall("sync"),
			AllowSyscall("sync_file_range"),
			AllowSyscall("syncfs"),
			AllowSyscall("sysinfo"),
			AllowSyscall("syslog"),
			AllowSyscall("tee"),
			AllowSyscall("tgkill"),
			AllowSyscall("time"),
			AllowSyscall("timer_create"),
			AllowSyscall("timer_delete"),
			AllowSyscall("timerfd_create"),
			AllowSyscall("timerfd_gettime"),
			AllowSyscall("timerfd_settime"),
			AllowSyscall("timer_getoverrun"),
			AllowSyscall("timer_gettime"),
			AllowSyscall("timer_settime"),
			AllowSyscall("times"),
			AllowSyscall("tkill"),
			AllowSyscall("truncate"),
			AllowSyscall("truncate64"),
			AllowSyscall("ugetrlimit"),
			AllowSyscall("umask"),
			AllowSyscall("uname"),
			AllowSyscall("unlink"),
			AllowSyscall("unlinkat"),
			AllowSyscall("utime"),
			AllowSyscall("utimensat"),
			AllowSyscall("utimes"),
			AllowSyscall("vfork"),
			AllowSyscall("vmsplice"),
			AllowSyscall("wait4"),
			AllowSyscall("waitid"),
			AllowSyscall("waitpid"),
			AllowSyscall("write"),
			AllowSyscall("writev"),
			AllowSyscall("arch_prctl"),
			AllowSyscall("modify_ldt"),
			AllowSyscall("chroot"),
			AllowSyscall("clone",
				specs.LinuxSeccompArg{
					Index:    0,
					Value:    2080505856,
					ValueTwo: 0,
					Op:       specs.OpMaskedEqual,
				},
			),
		},
	}

	kernelMinVersionChecker := NewKernelMinVersionChecker(sysctl.New())
	ptraceAllowed, err := kernelMinVersionChecker.CheckVersionIsAtLeast(MinPtraceKernelVersion())
	if err != nil {
		return nil, err
	}

	if ptraceAllowed {
		seccomp.Syscalls = append(
			seccomp.Syscalls,
			AllowSyscall("ptrace"),
		)
	}

	return seccomp, nil
}

func AllowSyscall(syscall string, args ...specs.LinuxSeccompArg) specs.LinuxSyscall {
	return specs.LinuxSyscall{
		Names:  []string{syscall},
		Action: specs.ActAllow,
		Args:   args,
	}
}
