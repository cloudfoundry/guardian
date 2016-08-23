/*
 * This executable passes through to the host's tar, extracting into a
 * directory in the container.
 *
 * It does this with a funky dance involving switching to the container's mount
 * namespace, creating the destination and saving off its fd, and then
 * switching back to the host's rootfs (but the container's destination) for
 * the actual untarring.
 */

#include <stdio.h>
#include <errno.h>
#include <string.h>
#include <sys/param.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <sys/syscall.h>
#include <unistd.h>
#include <linux/sched.h>
#include <linux/fcntl.h>

#include "pwd.h"

/* create a directory; chown only if newly created */
int mkdir_as(const char *dir, uid_t uid, gid_t gid) {
  int rv;

  rv = mkdir(dir, 0755);
  if(rv == 0) {
    /* new directory; set ownership */
    return chown(dir, uid, gid);
  } else {
    if(errno == EEXIST) {
      /* if directory already exists, leave ownership as-is */
      return 0;
    } else {
      /* if any other error, abort */
      return rv;
    }
  }

  /* unreachable */
  return -1;
}

/* recursively mkdir with directories owned by a given user */
int mkdir_p_as(const char *dir, uid_t uid, gid_t gid) {
  char tmp[PATH_MAX];
  char *p = NULL;
  size_t len;
  int rv;

  /* copy the given dir as it'll be mutated */
  snprintf(tmp, sizeof(tmp), "%s", dir);
  len = strlen(tmp);

  /* strip trailing slash */
  if(tmp[len - 1] == '/')
    tmp[len - 1] = 0;

  for(p = tmp + 1; *p; p++) {
    if(*p == '/') {
      /* temporarily null-terminte the string so that mkdir only creates this
       * path segment */
      *p = 0;

      /* mkdir with truncated path segment */
      rv = mkdir_as(tmp, uid, gid);
      if(rv == -1) {
        return rv;
      }

      /* restore path separator */
      *p = '/';
    }
  }

  /* create final destination */
  return mkdir_as(tmp, uid, gid);
}


#ifndef execveat
/**
 * We need to define execveat here since glibc does not provide a wrapper
 * for this syscall yet. This code will not run once glibc implements this.
 */
#if defined (__PPC64__)
#define EXECVEAT_CODE 362
#else
#define EXECVEAT_CODE 322
#endif
int execveat(int fd, const char *path, char **argv, char **envp, int flags) {
    return syscall(EXECVEAT_CODE, fd, path, argv, envp, flags);
}
#endif

/* nothing seems to define this... */
int setns(int fd, int nstype);

int main(int argc, char **argv) {
  int rv;
  int mntnsfd;
  int usrnsfd;
  char *user = NULL;
  char *destination = NULL;
  int tpid;
  int containerworkdir;
  char *tarpath;
  int tarfd;
  char *compress = NULL;
  struct passwd *pw;

  if(argc < 5) {
    fprintf(stderr, "Usage: %s <tar path> <wshd pid> <user> <destination> [files to compress]\n", argv[0]);
    return 1;
  }

  tarpath = argv[1];

  rv = sscanf(argv[2], "%d", &tpid);
  if(rv != 1) {
    fprintf(stderr, "invalid pid\n");
    return 1;
  }

  user = argv[3];
  destination = argv[4];

  if(argc > 5) {
    compress = argv[5];
  }

  char mntnspath[PATH_MAX];
  rv = snprintf(mntnspath, sizeof(mntnspath), "/proc/%u/ns/mnt", tpid);
  if(rv == -1) {
    perror("snprintf ns mnt path");
    return 1;
  }

  mntnsfd = open(mntnspath, O_RDONLY);
  if(mntnsfd == -1) {
    perror("open mnt namespace");
    return 1;
  }

  tarfd = open(tarpath, O_RDONLY|O_CLOEXEC);
  if(tarfd == -1) {
    perror("open host rootfs tar");
    return 1;
  }

  char usrnspath[PATH_MAX];
  rv = snprintf(usrnspath, sizeof(usrnspath), "/proc/%u/ns/user", tpid);
  if(rv == -1) {
    perror("snprintf ns user path");
    return 1;
  }

  usrnsfd = open(usrnspath, O_RDONLY);
  if(usrnsfd == -1) {
    perror("open user namespace");
    return 1;
  }

  /* switch to container's mount namespace/rootfs */
  rv = setns(mntnsfd, CLONE_NEWNS);
  if(rv == -1) {
    perror("setns");
    return 1;
  }
  close(mntnsfd);

  /* switch to container's user namespace so that user lookup returns correct uids */
  /* we allow this to fail if the container isn't user-namespaced */
  setns(usrnsfd, CLONE_NEWUSER);

  pw = getpwnam(user);
  if(pw == NULL) {
    perror("getpwnam");
    return 1;
  }

  rv = chdir(pw->pw_dir);
  if(rv == -1) {
    perror("chdir to user home");
    return 1;
  }

  rv = setgid(0);
  if(rv == -1) {
    perror("setgid");
    return 1;
  }

  rv = setuid(0);
  if(rv == -1) {
    perror("setuid");
    return 1;
  }

  /* create destination directory */
  rv = mkdir_p_as(destination, pw->pw_uid, pw->pw_gid);
  if(rv == -1) {
    char msg[1024];
    sprintf(msg, "mkdir_p_as %d %d", pw->pw_uid, pw->pw_gid);
    perror(msg);
    return 1;
  }

  /* save off destination dir for switching back to it later */
  containerworkdir = open(destination, O_RDONLY);
  if(containerworkdir == -1) {
    perror("open container destination");
    return 1;
  }

  /* switch to container's destination directory, with host still as rootfs */
  rv = fchdir(containerworkdir);
  if(rv == -1) {
    perror("fchdir to container destination");
    return 1;
  }

  rv = close(containerworkdir);
  if(rv == -1) {
    perror("close container destination");
    return 1;
  }

  rv = setgid(pw->pw_gid);
  if(rv == -1) {
    perror("setgid");
    return 1;
  }

  rv = setuid(pw->pw_uid);
  if(rv == -1) {
    perror("setuid");
    return 1;
  }

  if(compress != NULL) {
    rv = execveat(tarfd, "", (char*[5]){"tar", "cf", "-", compress, NULL}, (char*[0]){}, AT_EMPTY_PATH);
    if(rv == -1) {
      perror("execveat");
      return 1;
    }
  } else {
    rv = execveat(tarfd, "", (char*[4]){"tar", "xf", "-", NULL}, (char*[0]){}, AT_EMPTY_PATH);
    if(rv == -1) {
      perror("execveat");
      return 1;
    }
  }

  // unreachable
  return 2;
}
