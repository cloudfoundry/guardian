package diegonats_test

import (
	"testing"

	"github.com/cloudfoundry/gunk/diegonats"
)

func FunctionTakingNATSClient(diegonats.NATSClient) {

}

func TestCanPassFakeYagnatsAsNATSClient(t *testing.T) {
	FunctionTakingNATSClient(diegonats.NewFakeClient())
}
