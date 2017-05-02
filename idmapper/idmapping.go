package idmapper

import (
	"fmt"
	"strings"

	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type MappingList []specs.LinuxIDMapping

// MappingsForUser returns a slice of LinuxIDMapping covering the id range
// 0 -> maxID (typically 4294967294).
// When passed id = 0 (aka root), the container root id is mapped to
// the Host maxID.
// when passed id > 0 (aka non-root), the container root id is mapped to the
// non-root user's id.
// In both cases all ids in the entire id range (0 -> maxID) are mapped, with the
// maxID being omitted inside the user namespace.
func MappingsForUser(id, maxID uint32) MappingList {
	if id == 0 {
		return MappingList{
			{
				ContainerID: 0,
				HostID:      maxID,
				Size:        1,
			},
			{
				ContainerID: 1,
				HostID:      1,
				Size:        maxID - 1,
			},
		}
	}

	mappings := MappingList{{ContainerID: 0, HostID: id, Size: 1}}
	if id != 1 {
		mappings = append(mappings, specs.LinuxIDMapping{
			ContainerID: 1,
			HostID:      1,
			Size:        id - 1,
		})
	}
	if id != maxID {
		mappings = append(mappings, specs.LinuxIDMapping{
			ContainerID: id,
			HostID:      id + 1,
			Size:        maxID - id,
		})
	}
	return mappings
}

func (m MappingList) Map(id int) int {
	for _, m := range m {
		if delta := id - int(m.ContainerID); delta < int(m.Size) {
			return int(m.HostID) + delta
		}
	}

	return id
}

func (m MappingList) String() string {
	if len(m) == 0 {
		return "empty"
	}

	var parts []string
	for _, entry := range m {
		parts = append(parts, fmt.Sprintf("%d-%d-%d", entry.ContainerID, entry.HostID, entry.Size))
	}

	return strings.Join(parts, ",")
}
