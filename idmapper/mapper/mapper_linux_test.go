package mapper_test

import (
	"os/user"

	"code.cloudfoundry.org/idmapper/mapper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mapper", func() {
	var (
		desiredMappings []mapper.Mapping
		owner           *user.User
		idMapper        *mapper.IDMapper
		allowedSubids   map[string]mapper.Subid
	)

	BeforeEach(func() {
		desiredMappings = []mapper.Mapping{
			mapper.Mapping{
				ContainerID: 0,
				HostID:      1000,
				Size:        1,
			},
		}

		owner = &user.User{
			Uid:      "1000",
			Gid:      "1000",
			Username: "groot",
		}

		allowedSubids = make(map[string]mapper.Subid)
		allowedSubids[owner.Username] = mapper.Subid{
			Start: 100000,
			Size:  65000,
		}
	})

	JustBeforeEach(func() {
		idMapper = mapper.NewIDMapper(owner, desiredMappings, allowedSubids)
	})

	Describe("Parse", func() {
		It("returns a list of mapping", func() {
			mappings := mapper.Parse([]string{"1", "1000", "6500"})

			Expect(mappings).To(ConsistOf(mapper.Mapping{
				ContainerID: 1,
				HostID:      1000,
				Size:        6500,
			}))
		})
	})

	Describe("Map", func() {
		BeforeEach(func() {
			desiredMappings = append(desiredMappings,
				mapper.Mapping{
					ContainerID: 1002,
					HostID:      100000,
					Size:        65000,
				},
			)
		})

		It("returns desired mappings in the correct format", func() {
			data := idMapper.Map()
			Expect(string(data)).To(Equal("         0       1000          1\n      1002     100000      65000\n"))
		})
	})

	Describe("Validate", func() {
		It("is allowed to map any user to the owner", func() {
			err := idMapper.Validate()
			Expect(err).ToNot(HaveOccurred())
		})

		Context("when range is inside the allowed subids list", func() {
			BeforeEach(func() {
				desiredMappings = append(desiredMappings,
					mapper.Mapping{
						ContainerID: 1002,
						HostID:      100000,
						Size:        65000,
					},
				)
			})

			It("is allowed", func() {
				err := idMapper.Validate()
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when the range is zero", func() {
			BeforeEach(func() {
				desiredMappings = append(desiredMappings,
					mapper.Mapping{
						ContainerID: 1,
						HostID:      1001,
						Size:        0,
					},
				)
			})

			It("is not allowed", func() {
				err := idMapper.Validate()
				Expect(err).To(MatchError("mapping 1:1001:0 invalid: size can't be zero"))
			})
		})

		Context("when the owner isn't listed in the allowed ranges", func() {
			BeforeEach(func() {
				desiredMappings = append(desiredMappings,
					mapper.Mapping{
						ContainerID: 1002,
						HostID:      200000,
						Size:        65000,
					},
				)
			})

			It("is not allowed", func() {
				err := idMapper.Validate()
				Expect(err).To(MatchError("mapping 1002:200000:65000 invalid: range is not allowed"))
			})
		})

		Context("when the desired range is not allowed", func() {
			BeforeEach(func() {
				desiredMappings = append(desiredMappings,
					mapper.Mapping{
						ContainerID: 1002,
						HostID:      100001,
						Size:        65000,
					},
				)
			})

			It("is not allowed", func() {
				err := idMapper.Validate()
				Expect(err).To(MatchError("mapping 1002:100001:65000 invalid: range is not allowed"))
			})
		})
	})
})
