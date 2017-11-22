#include <unistd.h>

int main(int argc, char *argv[])
{
  int year = 3600 * 24 * 365;
  while (1) {
    sleep(year);
  }
}
