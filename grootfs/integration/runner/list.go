package runner

import (
	"bufio"
	"bytes"

	"code.cloudfoundry.org/grootfs/groot"
)

func (r Runner) List() ([]groot.Image, error) {
	imagePaths, err := r.RunSubcommand("list")
	if err != nil {
		return []groot.Image{}, err
	}

	images := []groot.Image{}
	buffer := bytes.NewBufferString(imagePaths)
	scanner := bufio.NewScanner(buffer)

	for scanner.Scan() {
		imagePath := scanner.Text()
		image := groot.Image{
			Path: imagePath,
		}
		images = append(images, image)
	}

	return images, nil
}
