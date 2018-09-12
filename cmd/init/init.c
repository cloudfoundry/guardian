#include <unistd.h>
#include "ignore_sigchild.h"

/* Tested by gqt/cmd/test_init.c */
int main(void) {
  set_not_wait_on_child();

  while (1) {
    pause();
  }
}
