package diegonats_test

import (
	"fmt"

	"github.com/apcera/nats"
	. "github.com/cloudfoundry/gunk/diegonats"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("NatsClient", func() {
	var natsClient NATSClient
	var natsUrls []string

	BeforeEach(func() {
		startNATS()
		natsUrls = []string{fmt.Sprintf("nats://127.0.0.1:%d", natsPort)}
		natsClient = NewClient()
	})

	AfterEach(func() {
		stopNATS()
	})

	Describe("Connect", func() {
		It("returns an error when connecting to an invalid address", func() {
			_, err := natsClient.Connect([]string{"nats://cats:bats@127.0.0.1:4223"})

			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).To(Equal("nats: No servers available for connection"))
		})
	})

	Describe("Subscription", func() {
		BeforeEach(func() {
			_, err := natsClient.Connect(natsUrls)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			if natsClient != nil {
				natsClient.Close()
			}
		})

		It("can subscribe/unsubscribe", func() {
			payload1 := make(chan []byte)
			payload2 := make(chan []byte)

			sid1, _ := natsClient.Subscribe("some.subject", func(msg *nats.Msg) {
				payload1 <- msg.Data
			})

			natsClient.Subscribe("some.subject", func(msg *nats.Msg) {
				payload2 <- msg.Data
			})

			natsClient.Publish("some.subject", []byte("hello!"))

			Eventually(payload1).Should(Receive(Equal([]byte("hello!"))))
			Eventually(payload2).Should(Receive(Equal([]byte("hello!"))))

			natsClient.Unsubscribe(sid1)

			natsClient.Publish("some.subject", []byte("hello!"))

			Consistently(payload1).ShouldNot(Receive())
			Eventually(payload2).Should(Receive(Equal([]byte("hello!"))))
		})

		It("can subscribe/unsubscribe with a queue", func() {
			payload := make(chan []byte)

			natsClient.QueueSubscribe("some.subject", "some-queue", func(msg *nats.Msg) {
				payload <- msg.Data
			})

			natsClient.QueueSubscribe("some.subject", "some-queue", func(msg *nats.Msg) {
				payload <- msg.Data
			})

			natsClient.Publish("some.subject", []byte("hello!"))

			Eventually(payload).Should(Receive(Equal([]byte("hello!"))))
			Consistently(payload).ShouldNot(Receive())
		})

		It("can subscribe/unsubscribe with a request/response", func() {
			payload := make(chan []byte)

			natsClient.Subscribe("some.request", func(msg *nats.Msg) {
				natsClient.Publish(msg.Reply, []byte("response!"))
			})

			natsClient.Subscribe("some.reply", func(msg *nats.Msg) {
				payload <- msg.Data
			})

			natsClient.PublishRequest("some.request", "some.reply", []byte("hello!"))

			Eventually(payload).Should(Receive(Equal([]byte("response!"))))
		})
	})
})
