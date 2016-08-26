package image_puller_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestImagePuller(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Image puller suite")
}

func ImagePathToVolumeID(imagePath string) string {
	stat, err := os.Stat(imagePath)
	Expect(err).ToNot(HaveOccurred())

	imagePathSha := sha256.Sum256([]byte(imagePath))
	return fmt.Sprintf("%s-%d", hex.EncodeToString(imagePathSha[:32]), stat.ModTime().Nanosecond())
}
