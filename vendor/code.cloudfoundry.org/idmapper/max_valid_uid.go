package idmapper

import (
	"bufio"
	"fmt"
	"os"
)

type IDMap string

const DefaultUIDMap IDMap = "/proc/self/uid_map"
const DefaultGIDMap IDMap = "/proc/self/gid_map"

const maxInt = int(^uint(0) >> 1)

func MustGetMaxValidUID() int {
	return must(DefaultUIDMap.MaxValid())
}

func MustGetMaxValidGID() int {
	return must(DefaultGIDMap.MaxValid())
}

func (u IDMap) MaxValid() (int, error) {
	f, err := os.Open(string(u))
	if err != nil {
		return 0, err
	}

	var m uint
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var container, host, size uint
		if _, err := fmt.Sscanf(scanner.Text(), "%d %d %d", &container, &host, &size); err != nil {
			return 0, ParseError{Line: scanner.Text(), Err: err}
		}

		m = minUint(maxUint(m, container+size-1), uint(maxInt))
	}

	return int(m), nil
}

func Min(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}

	return b
}

func maxUint(a, b uint) uint {
	if a > b {
		return a
	}

	return b
}

func minUint(a, b uint) uint {
	if a < b {
		return a
	}

	return b
}

type ParseError struct {
	Line string
	Err  error
}

func (p ParseError) Error() string {
	return fmt.Sprintf(`%s while parsing line "%s"`, p.Err, p.Line)
}

func must(a int, err error) int {
	if err != nil {
		panic(err)
	}

	return a
}
