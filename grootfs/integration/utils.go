package integration

import (
	"os/exec"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func CreateBundle(grootFSBin, graphPath, imagePath, id string) string {
	cmd := exec.Command(grootFSBin, "--graph", graphPath, "create", "--image", imagePath, id)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	return strings.TrimSpace(string(sess.Out.Contents()))
}

func FindUID(user string) uint32 {
	sess, err := gexec.Start(exec.Command("id", "-u", user), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	i, err := strconv.ParseInt(strings.TrimSpace(string(sess.Out.Contents())), 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return uint32(i)
}

func FindGID(group string) uint32 {
	sess, err := gexec.Start(exec.Command("id", "-g", group), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	i, err := strconv.ParseInt(strings.TrimSpace(string(sess.Out.Contents())), 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return uint32(i)
}
