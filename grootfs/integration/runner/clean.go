package runner

import "strconv"

func (r *Runner) Clean(threshold uint64) error {
	args := []string{}
	if threshold > 0 {
		args = append(args, "--threshold-bytes", strconv.FormatUint(threshold, 10))
	}

	_, err := r.RunSubcommand("clean", args...)
	return err
}
