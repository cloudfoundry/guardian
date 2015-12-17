OPTIMIZATION?=-O0
DEBUG?=-g -ggdb -rdynamic

all: nstar

clean:
	rm -f *.o nstar

.PHONY: all clean header

nstar: nstar.o pwd.o
	$(CC) -static -o $@ $^

%.o: %.c
	$(CC) -c -Wall $(OPTIMIZATION) $(DEBUG) $<
