package runner

import "strconv"

func (r *Runner) Clean(threshold uint64) (string, error) {
	args := []string{}
	if threshold > 0 {
		args = append(args, "--threshold-bytes", strconv.FormatUint(threshold, 10))
	}

	return r.RunSubcommand("clean", args...)
}
