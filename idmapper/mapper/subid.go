package mapper

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Subid struct {
	Start int
	Size  int
}

// Subid represents the values in the `/subuid` file
type Subids map[string]Subid

func LoadSubids(path string) (Subids, error) {
	subidFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	subids := make(map[string]Subid)
	scanner := bufio.NewScanner(subidFile)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")

		start, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid start value: %s", err)
		}
		size, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid size value: %s", err)
		}

		subids[parts[0]] = Subid{
			Start: start,
			Size:  size,
		}
	}

	return subids, nil
}
