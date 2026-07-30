package config_test

import (
	"os"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

var CurrentUserID string

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)

	SynchronizedBeforeSuite(func() []byte {
		return []byte{}
	}, func(data []byte) {
		userID := os.Getuid()
		CurrentUserID = strconv.Itoa(userID)
	})

	RunSpecs(t, "Config Suite")
}
