#include <stddef.h>
#include <signal.h>

/* Set not wait on child causes all children of the current process
 * not to become zombies on termination, removing the need to have a reaper.
 * As in notes of: http://man7.org/linux/man-pages/man2/waitpid.2.html */
void set_not_wait_on_child() {
  int action_success;
  struct sigaction chld_action;

  action_success = sigaction(SIGCHLD, NULL, &chld_action);
  chld_action.sa_flags |= SA_NOCLDWAIT;
  action_success = sigaction(SIGCHLD, &chld_action, NULL);
}

