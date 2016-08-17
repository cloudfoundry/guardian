package integration

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/grootfs/groot"
	"code.cloudfoundry.org/grootfs/store"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

func CreateBundle(grootFSBin, storePath, imagePath, id string) groot.Bundle {
	return CreateBundleWSpec(grootFSBin, storePath, groot.CreateSpec{
		ID:    id,
		Image: imagePath,
	})
}

func CreateBundleWSpec(grootFSBin, storePath string, spec groot.CreateSpec) groot.Bundle {
	args := []string{"--store", storePath, "create"}
	args = append(args, "--image", spec.Image)
	for _, mapping := range spec.UIDMappings {
		args = append(args, "--uid-mapping", fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size))
	}
	for _, mapping := range spec.GIDMappings {
		args = append(args, "--gid-mapping", fmt.Sprintf("%d:%d:%d", mapping.NamespaceID, mapping.HostID, mapping.Size))
	}
	args = append(args, spec.ID)

	cmd := exec.Command(grootFSBin, args...)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Expect(sess.Wait(10 * time.Second)).To(gexec.Exit(0))

	return store.NewBundle(strings.TrimSpace(string(sess.Out.Contents())))
}

func DeleteBundle(grootFSBin, storePath, id string) string {
	cmd := exec.Command(grootFSBin, "--store", storePath, "delete", id)
	sess, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).ToNot(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))
	return string(sess.Out.Contents())
}

func FindUID(user string) uint32 {
	sess, err := gexec.Start(exec.Command("id", "-u", user), nil, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	i, err := strconv.ParseInt(strings.TrimSpace(string(sess.Out.Contents())), 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return uint32(i)
}

func FindGID(group string) uint32 {
	sess, err := gexec.Start(exec.Command("id", "-g", group), nil, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(sess).Should(gexec.Exit(0))

	i, err := strconv.ParseInt(strings.TrimSpace(string(sess.Out.Contents())), 10, 32)
	Expect(err).NotTo(HaveOccurred())

	return uint32(i)
}
