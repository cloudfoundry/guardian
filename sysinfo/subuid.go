package sysinfo

import (
	"fmt"
	"strings"
)

func UidCanMapExactRange(subidFileContents string, username string, uid, subID, mapSize uint32) bool {
	for _, subidEntry := range strings.Split(subidFileContents, "\n") {
		if subidEntry == fmt.Sprintf("%d:%d:%d", uid, subID, mapSize) {
			return true
		}
		if subidEntry == fmt.Sprintf("%s:%d:%d", username, subID, mapSize) {
			return true
		}
	}

	return false
}
