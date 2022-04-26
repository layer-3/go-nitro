// Package testactors exports peers with vanity addresses: with corresponding keys, names and virtual funding protocol roles.
package testactors

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/statechannels/go-nitro/types"
)

type Actor struct {
	Address    types.Address
	PrivateKey []byte
	Role       uint
	Name       string
}

func (a Actor) Destination() types.Destination {
	return types.AddressToDestination(a.Address)
}

var Alice Actor = Actor{
	common.HexToAddress(`0xAAA6628Ec44A8a742987EF3A114dDFE2D4F7aDCE`),
	common.Hex2Bytes(`2d999770f7b5d49b694080f987b82bbc9fc9ac2b4dcc10b0f8aba7d700f69c6d`),
	0,
	"alice",
}
var Bob Actor = Actor{
	common.HexToAddress(`0xBBB676f9cFF8D242e9eaC39D063848807d3D1D94`),
	common.Hex2Bytes(`0279651921cd800ac560c21ceea27aab0107b67daf436cdd25ce84cad30159b4`),
	2,
	"bob",
}
var Brian Actor = Actor{
	common.HexToAddress("0xB2B22ec3889d11f2ddb1A1Db11e80D20EF367c01"),
	common.Hex2Bytes("0aca28ba64679f63d71e671ab4dbb32aaa212d4789988e6ca47da47601c18fe2"),
	2,
	"brian",
}
var Irene Actor = Actor{
	common.HexToAddress(`0x111A00868581f73AB42FEEF67D235Ca09ca1E8db`),
	common.Hex2Bytes(`febb3b74b0b52d0976f6571d555f4ac8b91c308dfa25c7b58d1e6a7c3f50c781`),
	1,
	"irene",
}