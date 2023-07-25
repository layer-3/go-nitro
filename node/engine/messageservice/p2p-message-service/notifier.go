package p2pms

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

type NetworkNotifiee struct {
	ms *P2PMessageService
}

func (nn *NetworkNotifiee) Connected(n network.Network, conn network.Conn) {
	nn.ms.logger.Debug().Msgf("notification: connected to peer %s", conn.RemotePeer().Pretty())
	go nn.ms.sendPeerInfo(conn.RemotePeer(), false)
}

func (nn NetworkNotifiee) Disconnected(n network.Network, conn network.Conn) {
	nn.ms.logger.Debug().Msgf("notification: disconnected from peer: %s", conn.RemotePeer().Pretty())
}

func (nn NetworkNotifiee) Listen(network.Network, multiaddr.Multiaddr)      {}
func (nn NetworkNotifiee) ListenClose(network.Network, multiaddr.Multiaddr) {}
