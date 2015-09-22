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

	var (
		bndle *rundmc.Bundle
		tmp   string
	)

	BeforeEach(func() {
		var err error
		tmp, err = ioutil.TempDir("", "bndle")
		Expect(err).NotTo(HaveOccurred())

		bndle = rundmc.BundleForCmd(exec.Command("echo", "hello"))
		Expect(bndle.Create(tmp)).To(Succeed())
	})

	AfterEach(func() {
		if tmp != "" {
			os.RemoveAll(tmp)
		}
	})

	Context("config.json", func() {
		var (
			config map[string]interface{}
		)

		BeforeEach(func() {
			config = parseJson(path.Join(tmp, "config.json"))
		})

		It("should contain the specified command", func() {
			Expect(config["process"]).To(HaveKeyWithValue(
				BeEquivalentTo("args"),
				ConsistOf("echo", "hello"),
			))
		})
	})

	Context("runtime.json", func() {
		var (
			runtime map[string]interface{}
		)

		BeforeEach(func() {
			runtime = parseJson(path.Join(tmp, "runtime.json"))
		})

		Context("linux", func() {
			var linux map[string]interface{}

			BeforeEach(func() {
				var ok bool
				linux, ok = runtime["linux"].(map[string]interface{})
				Expect(ok).To(BeTrue())
			})

			It("should configure all the namespaces", func() {
				Expect(linux).To(HaveKey(
					BeEquivalentTo("namespaces"),
				))
				namespaces, ok := linux["namespaces"].([]interface{})
				Expect(ok).To(BeTrue())
				Expect(namespaces).To(ConsistOf(expectedNamespaces()...))
			})
		})

	})
})

func expectedNamespaces() []interface{} {
	namespaces := make([]interface{}, 6)
	nsTypes := []interface{}{"pid", "network", "mount", "ipc", "uts", "user"}
	for i, NsType := range nsTypes {
		nsMap := make(map[string]interface{}, 2)
		nsMap["type"], nsMap["path"] = NsType, ""
		namespaces[i] = nsMap
	}
	return namespaces
}

func parseJson(path string) map[string]interface{} {
	Expect(path).To(BeAnExistingFile())
	configFile, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	config := make(map[string]interface{})
	Expect(json.NewDecoder(configFile).Decode(&config)).To(Succeed())
	return config
}
