package runner

import (
	"bufio"
	"bytes"

	"code.cloudfoundry.org/grootfs/groot"
)

func (r Runner) List() ([]groot.ImageInfo, error) {
	imagePaths, err := r.RunSubcommand("list")
	if err != nil {
		return []groot.ImageInfo{}, err
	}

	images := []groot.ImageInfo{}
	buffer := bytes.NewBufferString(imagePaths)
	scanner := bufio.NewScanner(buffer)

	for scanner.Scan() {
		imagePath := scanner.Text()
		image := groot.ImageInfo{
			Path: imagePath,
		}
		images = append(images, image)
	}

	return images, nil
}
