package rundmc_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BundleForCmd", func() {
	It("creates a bundle with the specified command", func() {
		tmp, err := ioutil.TempDir("", "bndle")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmp)

		bndle := rundmc.BundleForCmd(exec.Command("echo", "hello"))
		bndle.Create(tmp)

		configFile, err := os.Open(path.Join(tmp, "config.json"))
		Expect(err).NotTo(HaveOccurred())

		config := make(map[string]interface{})
		Expect(json.NewDecoder(configFile).Decode(&config)).To(Succeed())

		Expect(config["process"]).To(HaveKeyWithValue(
			BeEquivalentTo("args"),
			ConsistOf("echo", "hello"),
		))
	})
})
