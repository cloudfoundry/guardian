package runner

import "strconv"

func (r Runner) Clean(threshold int64, ignoredImages []string) (string, error) {
	args := []string{}

	args = append(args, "--threshold-bytes", strconv.FormatInt(threshold, 10))

	for _, ignoredImage := range ignoredImages {
		args = append(args, "--ignore-image", ignoredImage)
	}

	return r.RunSubcommand("clean", args...)
}
