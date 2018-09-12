#include <unistd.h>
#include "ignore_sigchild.h"

int main(void) {
  set_not_wait_on_child();

  if (fork() == 0) {
    sleep(1);
  } else {
    while(1) {
      pause();
    }
  }
}
