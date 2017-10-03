package runner

import "strconv"

func (r Runner) Clean(cacheSize int64) (string, error) {
	args := []string{}

	args = append(args, "--cache-bytes", strconv.FormatInt(cacheSize, 10))

	return r.RunSubcommand("clean", args...)
}
