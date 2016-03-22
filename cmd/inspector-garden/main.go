package main

/*
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <errno.h>
#include <fcntl.h>
#include <sys/param.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <unistd.h>
#include <linux/sched.h>

int setns(int fd, int nstype);

void enterns() {
	if (!getenv("TARGET_NS_PID")) {
		return;
	}
  int rv;
  int mntnsfd;

  char mntnspath[PATH_MAX];
  rv = snprintf(mntnspath, sizeof(mntnspath), "/proc/%s/ns/mnt", getenv("TARGET_NS_PID"));
  if(rv == -1) {
    perror("Could not enter mnt space");
		exit(1);
  }

  mntnsfd = open(mntnspath, O_RDONLY);
  if(mntnsfd == -1) {
    perror("Could not enter the namespace");
		exit(1);
  }

  rv = setns(mntnsfd, CLONE_NEWNS);
  if(rv == -1) {
    perror("Could not setns");
		exit(1);
  }
  close(mntnsfd);
}

__attribute__((constructor)) void init(void) {
	enterns();
}

*/
import "C"

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

func main() {
	pid := flag.Int("pid", -1, "garden pid process")
	flag.Parse()

	if *pid == -1 {
		flag.Usage()
		os.Exit(1)
	}

	if isGraphPathVisible() {
		executeUserProgram(flag.Args())
	} else {
		enterPidNamespace(*pid)
	}
}

func isGraphPathVisible() bool {
	if os.Getenv("TARGET_NS_PID") == "" {
		return false
	}

	return true
}

func executeUserProgram(args []string) {
	program := "/bin/sh"
	if len(args) >= 1 {
		program = args[0]
	}

	ps1 := "PS1=inspector-garden#"
	executeProgram(program, nil, append(os.Environ(), ps1))
}

// It is not allowed to reenter the caller's current namespace. Said that, we
// need to set the env variable and call the program again.
func enterPidNamespace(pid int) {
	envVars := append(os.Environ(), "TARGET_NS_PID="+strconv.Itoa(pid))
	executeProgram(os.Args[0], os.Args, envVars)
}

func executeProgram(program string, args []string, envVars []string) {
	pa := os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		Env:   envVars,
	}

	proc, err := os.StartProcess(program, args, &pa)
	if err != nil {
		fmt.Printf("Failed to execute the following program: `%s`. Error: %s .", program, err.Error())
		os.Exit(1)
	}

	// Wait until user exits the shell
	state, err := proc.Wait()
	if err != nil {
		fmt.Printf("Failed to execute the following program: `%s`. Error: %s .", program, err.Error())
		os.Exit(1)
	}

	if !state.Success() {
		os.Exit(1)
	}
}
