// Package client_test contains helpers and integration tests for go-nitro clients
package client_test // import "github.com/statechannels/go-nitro/client_test"

import (
	"testing"

	"github.com/statechannels/go-nitro/client"
	"github.com/statechannels/go-nitro/client/engine"
	"github.com/statechannels/go-nitro/client/engine/chainservice"
	"github.com/statechannels/go-nitro/client/engine/messageservice"
	"github.com/statechannels/go-nitro/client/engine/store"
	"github.com/statechannels/go-nitro/types"
)

func TestCrashTolerance(t *testing.T) {

	// Setup logging
	logFile := "test_crash_tolerance.log"
	truncateLog(logFile)
	logDestination := newLogWriter(logFile)

	// Setup chain service
	sim, bindings, ethAccounts, err := chainservice.SetupSimulatedBackend(3)
	if err != nil {
		t.Fatal(err)
	}

	chainA, err := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[0], logDestination)
	if err != nil {
		t.Fatal(err)
	}

	chainB, err := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[2], logDestination)
	if err != nil {
		t.Fatal(err)
	}
	// End chain service setup

	broker := messageservice.NewBroker()

	// Client setup
	storeA := store.NewMemStore(alice.PrivateKey)
	messageserviceA := messageservice.NewTestMessageService(alice.Address(), broker, 0)
	clientA := client.New(messageserviceA, chainA, storeA, logDestination, &engine.PermissivePolicy{}, nil)

	clientB, _ := setupClient(bob.PrivateKey, chainB, broker, logDestination, 0)
	// End Client setup

	// test successful condition for setup / teadown of unused ledger channel
	{
		channelId := directlyFundALedgerChannel(t, clientA, clientB, types.Address{})

		clientA.Close()
		anotherMessageserviceA := messageservice.NewTestMessageService(alice.Address(), broker, 0)
		anotherChainA, err := chainservice.NewSimulatedBackendChainService(sim, bindings, ethAccounts[0], logDestination)
		if err != nil {
			t.Fatal(err)
		}
		anotherClientA := client.New(
			anotherMessageserviceA,
			anotherChainA,
			storeA, logDestination, &engine.PermissivePolicy{}, nil)

		directlyDefundALedgerChannel(t, anotherClientA, clientB, channelId)

	}

}