package runner

import "strconv"

func (r *Runner) Clean(threshold uint64, ignoredImages []string) (string, error) {
	args := []string{}
	if threshold > 0 {
		args = append(args, "--threshold-bytes", strconv.FormatUint(threshold, 10))
	}

	for _, ignoredImage := range ignoredImages {
		args = append(args, "--ignore-image", ignoredImage)
	}

	return r.RunSubcommand("clean", args...)
}
