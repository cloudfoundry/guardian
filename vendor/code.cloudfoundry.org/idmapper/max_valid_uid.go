package idmapper

import (
	"bufio"
	"fmt"
	"os"
)

type IDMap string

const DefaultUIDMap IDMap = "/proc/self/uid_map"
const DefaultGIDMap IDMap = "/proc/self/gid_map"

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

	m := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var container, host, size int
		if _, err := fmt.Sscanf(scanner.Text(), "%d %d %d", &container, &host, &size); err != nil {
			return 0, ParseError{Line: scanner.Text(), Err: err}
		}

		m = Max(m, container+size-1)
	}

	return m, nil
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
