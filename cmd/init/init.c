#include <sys/types.h>
#include <sys/wait.h>
#include <stddef.h>
#include <errno.h>
#include <stdio.h>
#include <string.h>
#include <stdbool.h>
#include <signal.h>

#define len(array) (sizeof(array) / sizeof(array)[0])
#define no_children(harvest) (harvest == -1 && errno == ECHILD)

static bool reap() {
  int harvest;

  while (true) {
    harvest = wait(NULL);
    if (no_children(harvest)) return true;
    if (harvest == -1) {
      printf("failed to reap children: %s\n", strerror(errno));
      return false;
    }
  }
}

static bool configure_signals(sigset_t *set) {
  size_t i;
  int ignored_signals[] = {SIGSEGV, SIGABRT, SIGFPE, SIGILL, SIGSYS, SIGTTIN, SIGTTOU, SIGTRAP, SIGBUS};

  if (sigfillset(set) == -1) {
    printf("failed to configure signals: %s\n", strerror(errno));
    return false;
  }

  for (i = 0; i < len(ignored_signals); i++) {
    if (sigdelset(set, ignored_signals[i]) == -1) {
      printf("failed to configure signals: %s\n", strerror(errno));
      return false;
    }
  }

  if (sigprocmask(SIG_SETMASK, set, NULL) == -1) {
    printf("failed to configure signals: %s\n", strerror(errno));
    return false;
  }

  return true;
}

int main(void) {
  sigset_t set;
  int sig;

  if (!configure_signals(&set)) return 1;

  while (true) {
    if (sigwait(&set, &sig) != 0) {
      printf("failed to wait for signals: %s\n", strerror(errno));
      return 1;
    }
    if (!reap()) return 1;
  }
}
