package outcome

import (
	"math/big"
)

// min returns the minimum of the supplied integers as a new big.Int
func min(a *big.Int, b *big.Int) big.Int {
	switch a.Cmp(b) {
	case -1:
		return *big.NewInt(0).Set(a)
	default:
		return *big.NewInt(0).Set(b)
	}
}

// ComputeTransferEffectsAndInteractions computes the effects and interactions that will be executed on-chain when "transfer" is called
func ComputeTransferEffectsAndInteractions(initialHoldings big.Int, allocations Allocations, indices []uint) (newAllocations Allocations, exitAllocations Allocations) {
	var k uint
	surplus := big.NewInt(0).Set(&initialHoldings)
	newAllocations = make([]Allocation, len(allocations))
	exitAllocations = make([]Allocation, len(allocations))

	// for each allocation
	for i := 0; i < len(allocations); i++ {
		// copy allocation
		newAllocations[i] = Allocation{
			Destination:    allocations[i].Destination,
			Amount:         *big.NewInt(0).Set(&allocations[i].Amount),
			AllocationType: allocations[i].AllocationType,
			Metadata:       allocations[i].Metadata,
		}
		// compute payout amount
		affordsForDestination := min(&allocations[i].Amount, surplus)
		if len(indices) == 0 || k < uint(len(indices)) && indices[k] == uint(i) {
			// decrease allocation amount
			newAllocations[i].Amount.Sub(&newAllocations[i].Amount, &affordsForDestination)
			// increase exit allocation amount
			exitAllocations[i] = Allocation{
				Destination:    allocations[i].Destination,
				Amount:         *big.NewInt(0).Set(&affordsForDestination),
				AllocationType: allocations[i].AllocationType,
				Metadata:       allocations[i].Metadata,
			}
		}
		// decrease surplus
		surplus.Sub(surplus, &affordsForDestination)
	}

	return

}
