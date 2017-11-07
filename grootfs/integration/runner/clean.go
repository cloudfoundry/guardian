package runner

import "strconv"

func (r Runner) Clean(threshold int64) (string, error) {
	args := []string{}

	args = append(args, "--threshold-bytes", strconv.FormatInt(threshold, 10))

	return r.RunSubcommand("clean", args...)
}
