package sysinfo

import (
	"fmt"
	"strings"
)

func UidCanMapExactRange(subidFileContents string, uid, subID, mapSize uint32) bool {
	for _, subidEntry := range strings.Split(subidFileContents, "\n") {
		if subidEntry == fmt.Sprintf("%d:%d:%d", uid, subID, mapSize) {
			return true
		}
	}

	return false
}
