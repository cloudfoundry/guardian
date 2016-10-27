package mapper

import (
	"bytes"
	"fmt"
	"os/user"
	"strconv"
)

type Mapping struct {
	HostID      int
	ContainerID int
	Size        int
}

func Parse(args []string) []Mapping {
	m := []Mapping{}

	for i := 0; i < len(args)/3; i++ {
		containerID, _ := strconv.Atoi(args[i*3+0])
		hostID, _ := strconv.Atoi(args[i*3+1])
		size, _ := strconv.Atoi(args[i*3+2])

		mapping := Mapping{
			ContainerID: containerID,
			HostID:      hostID,
			Size:        size,
		}
		m = append(m, mapping)
	}

	return m
}

type IDMapper struct {
	Owner           *user.User
	DesiredMappings []Mapping
	AllowedSubids   Subids
}

func NewIDMapper(owner *user.User, desiredMappings []Mapping, allowedSubids Subids) *IDMapper {
	return &IDMapper{
		Owner:           owner,
		DesiredMappings: desiredMappings,
		AllowedSubids:   allowedSubids,
	}
}

func (m *IDMapper) Validate() error {
	for _, mapping := range m.DesiredMappings {
		if err := m.validateMapping(mapping); err != nil {
			return err
		}
	}

	return nil
}

func (m *IDMapper) validateMapping(mapping Mapping) error {
	if mapping.Size == 0 {
		return m.errorMessage(mapping, "size can't be zero")
	}

	hostID := strconv.Itoa(mapping.HostID)
	if mapping.Size == 1 && hostID == m.Owner.Uid {
		return nil
	}

	if subidRange, ok := m.AllowedSubids[m.Owner.Username]; ok {
		if mapping.HostID >= subidRange.Start && mapping.HostID+mapping.Size-1 <= (subidRange.Start+subidRange.Size)-1 {
			return nil
		}
	}

	return m.errorMessage(mapping, "range is not allowed")
}

func (m *IDMapper) Map() []byte {
	procMappings := bytes.NewBuffer([]byte{})

	for _, mapping := range m.DesiredMappings {
		procMappings.WriteString(fmt.Sprintf("%10d %10d %10d\n", mapping.ContainerID, mapping.HostID, mapping.Size))
	}

	return procMappings.Bytes()
}

func (m *IDMapper) errorMessage(mapping Mapping, message string) error {
	return fmt.Errorf("mapping %d:%d:%d invalid: %s",
		mapping.ContainerID,
		mapping.HostID,
		mapping.Size,
		message,
	)
}
