package runner

import (
	"encoding/json"

	"code.cloudfoundry.org/grootfs/groot"
)

func (r Runner) Stats(id string) (groot.VolumeStats, error) {
	stats, err := r.RunSubcommand("stats", id)
	if err != nil {
		return groot.VolumeStats{}, err
	}

	var volumeStats groot.VolumeStats
	err = json.Unmarshal([]byte(stats), &volumeStats)
	return volumeStats, err
}
