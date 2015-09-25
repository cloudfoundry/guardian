package rundmc_test

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"

	"github.com/cloudfoundry-incubator/guardian/rundmc"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("BundleForCmd", func() {

	var (
		bundle *rundmc.Bundle
		mounts []rundmc.Mount
		tmp    string
	)

	BeforeEach(func() {
		var err error
		tmp, err = ioutil.TempDir("", "bundle")
		Expect(err).NotTo(HaveOccurred())

		mounts = []rundmc.Mount{
			{
				Name:        "apple",
				Type:        "apple_fs",
				Source:      "iDevice",
				Destination: "/apple",
				Options: []string{
					"healthy",
					"shiny",
				},
			},
			{
				Name:        "banana",
				Type:        "banana_fs",
				Source:      "banana_device",
				Destination: "/banana",
				Options: []string{
					"yellow",
					"fresh",
				},
			},
		}
	})

	JustBeforeEach(func() {
		bundle = rundmc.BundleForCmd(
			exec.Command("echo", "hello"),
			mounts,
		)
		Expect(bundle.Create(tmp)).To(Succeed())
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

		JustBeforeEach(func() {
			config = parseJson(path.Join(tmp, "config.json"))
		})

		It("should contain the specified command", func() {
			Expect(config["process"]).To(HaveKeyWithValue(
				BeEquivalentTo("args"),
				ConsistOf("echo", "hello"),
			))
		})

		Describe("mounts", func() {
			DescribeTable("should mount",
				func(name, path string) {
					Expect(config["mounts"]).To(ContainElement(
						map[string]interface{}{
							"name": name,
							"path": path,
						},
					))
				},
				Entry("banana", "banana", "/banana"),
				Entry("apple", "apple", "/apple"),
			)
		})
	})

	Context("runtime.json", func() {
		var (
			runtime map[string]interface{}
		)

		JustBeforeEach(func() {
			runtime = parseJson(path.Join(tmp, "runtime.json"))
		})

		Describe("mounts", func() {
			DescribeTable("should mount",
				func(name, fsType, source string, options []string) {
					Expect(runtime).To(HaveKey("mounts"))
					mounts, _ := runtime["mounts"].(map[string]interface{})

					Expect(mounts).To(HaveKey(name))
					mount, _ := mounts[name].(map[string]interface{})

					Expect(mount).To(HaveKeyWithValue(
						BeEquivalentTo("type"), Equal(fsType),
					))
					Expect(mount).To(HaveKeyWithValue(
						BeEquivalentTo("source"), Equal(source),
					))
					for _, option := range options {
						Expect(mount).To(HaveKeyWithValue(
							BeEquivalentTo("options"), ContainElement(option),
						))
					}
				},
				Entry("banana", "banana", "banana_fs", "banana_device", []string{
					"yellow", "fresh",
				}),
				Entry("apple", "apple", "apple_fs", "iDevice", []string{
					"healthy", "shiny",
				}),
			)
		})

		Describe("linux", func() {
			var linux interface{}

			JustBeforeEach(func() {
				Expect(runtime).To(HaveKey("linux"))
				linux = runtime["linux"]
			})

			It("should configure all the namespaces", func() {
				Expect(linux).To(HaveKeyWithValue(
					BeEquivalentTo("namespaces"),
					ConsistOf(ns("network"), ns("pid"), ns("mount"), ns("ipc"), ns("uts")),
				))
			})

			It("should configure cgroups", func() {
				for _, key := range []string{"memory", "cpu", "pids", "blockIO", "hugepageLimit", "network"} {
					Expect(linux).To(HaveKeyWithValue(
						BeEquivalentTo("resources"),
						HaveKey(key),
					))
				}
			})
		})
	})
})

func ns(n string) map[string]interface{} {
	return map[string]interface{}{
		"type": n,
		"path": "",
	}
}

func parseJson(path string) map[string]interface{} {
	Expect(path).To(BeAnExistingFile())
	configFile, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())
	config := make(map[string]interface{})
	Expect(json.NewDecoder(configFile).Decode(&config)).To(Succeed())
	return config
}
