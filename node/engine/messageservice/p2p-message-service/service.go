package p2pms

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	p2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
	"github.com/statechannels/go-nitro/internal/logging"
	"github.com/statechannels/go-nitro/internal/safesync"
	"github.com/statechannels/go-nitro/protocols"
	"github.com/statechannels/go-nitro/types"
)

// basicPeerInfo contains the basic information about a peer
type basicPeerInfo struct {
	Id      peer.ID
	Address types.Address
}

type peerExchangeMessage struct {
	Id             peer.ID
	Address        types.Address
	ExpectResponse bool
}

const (
	DHT_PROTOCOL_PREFIX          protocol.ID = "/nitro" // use /nitro/kad/1.0.0 instead of /ipfs/kad/1.0.0
	PROTOCOL_ID                  protocol.ID = "/nitro/msg/1.0.0"
	PEER_EXCHANGE_PROTOCOL_ID    protocol.ID = "/nitro/peerinfo/1.0.0"
	DELIMITER                                = '\n'
	BUFFER_SIZE                              = 1_000
	NUM_CONNECT_ATTEMPTS                     = 10
	RETRY_SLEEP_DURATION                     = 5 * time.Second
	PEER_EXCHANGE_SLEEP_DURATION             = 10 * time.Second // how often we attempt FindPeers
	BOOTSTRAP_SLEEP_DURATION                 = 1 * time.Second  // how often we check for bootpeers in Peerstore
)

// P2PMessageService is a rudimentary message service that uses TCP to send and receive messages.
type P2PMessageService struct {
	toEngine chan protocols.Message // for forwarding processed messages to the engine
	peers    *safesync.Map[peer.ID]

	me          types.Address
	key         p2pcrypto.PrivKey
	p2pHost     host.Host
	mdns        mdns.Service
	dht         *dht.IpfsDHT
	newPeerInfo chan basicPeerInfo
	logger      zerolog.Logger

	MultiAddr string
}

// Id returns the libp2p peer ID of the message service.
func (ms *P2PMessageService) Id() peer.ID {
	id, _ := peer.IDFromPrivateKey(ms.key)
	return id
}

// NewMessageService returns a running P2PMessageService listening on the given ip, port and message key.
// If useMdnsPeerDiscovery is true, the message service will use mDNS to discover peers.
// Otherwise, peers must be added manually via `AddPeers`.
func NewMessageService(ip string, port int, me types.Address, pk []byte, useMdnsPeerDiscovery bool, logWriter io.Writer, bootPeers []string) *P2PMessageService {
	logging.ConfigureZeroLogger()

	ms := &P2PMessageService{
		toEngine:    make(chan protocols.Message, BUFFER_SIZE),
		newPeerInfo: make(chan basicPeerInfo, BUFFER_SIZE),
		peers:       &safesync.Map[peer.ID]{},
		me:          me,
		logger:      logging.WithAddress(zerolog.New(logWriter).With().Timestamp(), &me).Caller().Logger(),
	}

	messageKey, err := p2pcrypto.UnmarshalSecp256k1PrivateKey(pk)
	if err != nil {
		panic(err)
	}

	ms.key = messageKey
	options := []libp2p.Option{
		libp2p.Identity(messageKey),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", ip, port)),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.NATPortMap(),
		libp2p.DefaultMuxers,
	}
	host, err := libp2p.New(options...)
	if err != nil {
		panic(err)
	}

	ms.p2pHost = host
	ms.p2pHost.SetStreamHandler(PROTOCOL_ID, ms.msgStreamHandler)
	ms.p2pHost.SetStreamHandler(PEER_EXCHANGE_PROTOCOL_ID, ms.receivePeerInfo)

	// Print out my own peerInfo
	peerInfo := peer.AddrInfo{
		ID:    ms.p2pHost.ID(),
		Addrs: ms.p2pHost.Addrs(),
	}
	addrs, err := peer.AddrInfoToP2pAddrs(&peerInfo)
	if err != nil {
		panic(err)
	}
	ms.MultiAddr = addrs[0].String()
	ms.logger.Info().Msgf("libp2p node multiaddrs: %v", addrs)

	if useMdnsPeerDiscovery {
		err = ms.setupMdns()
	} else {
		err = ms.setupDht(bootPeers)
	}
	if err != nil {
		panic(err)
	}

	return ms
}

func (ms *P2PMessageService) setupMdns() error {
	// Since the mdns service could trigger a call to  `HandlePeerFound` at any time once started
	// We want to start mdns after the message service has been fully constructed
	ms.mdns = mdns.NewMdnsService(ms.p2pHost, "", ms)
	return ms.mdns.Start()
}

func (ms *P2PMessageService) setupDht(bootPeers []string) error {
	ctx := context.Background()
	var options []dht.Option
	options = append(options, dht.BucketSize(20))
	options = append(options, dht.Mode(dht.ModeServer))                                                    // allows other peers to connect to this node
	options = append(options, dht.ProtocolPrefix(DHT_PROTOCOL_PREFIX))                                     // need this to allow custom NamespacedValidator
	options = append(options, dht.NamespacedValidator(DHT_NAMESPACE, stateChannelAddrToPeerIDValidator{})) // all records prefixed with /scaddr/ will use this custom validator

	kademliaDHT, err := dht.New(ctx, ms.p2pHost, options...)
	if err != nil {
		return err
	}
	ms.dht = kademliaDHT

	// Setup notifications so we exchange nitro signing addresses when connected
	n := &network.NotifyBundle{}
	n.ConnectedF = func(n network.Network, conn network.Conn) {
		ms.logger.Debug().Msgf("notification: connected to peer %s", conn.RemotePeer().String())
		go ms.sendPeerInfo(conn.RemotePeer(), false)
	}
	n.DisconnectedF = func(n network.Network, conn network.Conn) {
		ms.logger.Debug().Msgf("notification: disconnected from peer: %s", conn.RemotePeer().String())
	}

	ms.p2pHost.Network().Notify(n)

	expectedPeers := len(bootPeers)
	if expectedPeers > 0 {
		// Add bootpeers and wait for connections before proceeding
		ms.addBootPeers(bootPeers)
		ms.logger.Info().Msgf("waiting for %d bootpeer connections", expectedPeers)
		ticker := time.NewTicker(BOOTSTRAP_SLEEP_DURATION)
		for range ticker.C {
			peers := ms.p2pHost.Network().Peers()
			actualPeers := len(peers)
			ms.logger.Debug().Msgf("found peers: %v, expected peers: %d", actualPeers, expectedPeers)
			for _, peer := range peers {
				ms.logger.Debug().Msgf("peer info: %v", peer)
			}

			// Once we've connected to enough peers, stop the ticker
			if actualPeers >= expectedPeers {
				ms.logger.Info().Msgf("initial threshold for peer connections has been met")
				ticker.Stop()
				break
			}
		}

		ms.addScaddrDhtRecord(ctx)
	}

	err = ms.dht.Bootstrap(ctx) // Runs periodically to maintain a healthy routing table
	if err != nil {
		return err
	}

	ms.logger.Info().Msgf("DHT setup complete")
	return nil
}

// addScaddrDhtRecord adds this node's state channel address to the custom dht namespace
func (ms *P2PMessageService) addScaddrDhtRecord(ctx context.Context) error {
	recordData := &dhtData{
		SCAddr:    ms.me.String(),
		PeerID:    ms.Id().String(),
		Timestamp: time.Time.Unix(time.Now()),
	}
	recordDataBytes, err := json.Marshal(recordData)
	if err != nil {
		return err
	}

	signature, err := ms.key.Sign(recordDataBytes)
	if err != nil {
		return err
	}

	fullRecord := &dhtRecord{
		Data:      *recordData,
		Signature: signature,
	}
	fullRecordBytes, err := json.Marshal(fullRecord)
	if err != nil {
		return err
	}

	key := DHT_RECORD_PREFIX + ms.me.String()
	err = ms.dht.PutValue(ctx, key, fullRecordBytes)
	if err != nil {
		return err
	}
	ms.logger.Info().Str(key, ms.Id().String()).Msg("Added value to dht")
	return nil
}

// HandlePeerFound is called by the mDNS service when a peer is found.
func (ms *P2PMessageService) HandlePeerFound(pi peer.AddrInfo) {
	ms.logger.Debug().Msgf("Attempting to add mdns peer")
	ms.p2pHost.Peerstore().AddAddr(pi.ID, pi.Addrs[0], peerstore.PermanentAddrTTL)

	ms.sendPeerInfo(pi.ID, false)
}

func (ms *P2PMessageService) msgStreamHandler(stream network.Stream) {
	defer stream.Close()

	reader := bufio.NewReader(stream)
	// Create a buffer stream for non blocking read and write.
	raw, err := reader.ReadString(DELIMITER)

	// An EOF means the stream has been closed by the other side.
	if errors.Is(err, io.EOF) {
		return
	}
	if err != nil {
		ms.logger.Err(err)
		return
	}
	m, err := protocols.DeserializeMessage(raw)
	if err != nil {
		ms.logger.Err(err)
		return
	}
	ms.toEngine <- m
}

// sendPeerInfo sends our peer info to a given peerId
// Triggered whenever node establishes a connection with a peer
func (ms *P2PMessageService) sendPeerInfo(recipientId peer.ID, expectResponse bool) {
	stream, err := ms.p2pHost.NewStream(context.Background(), recipientId, PEER_EXCHANGE_PROTOCOL_ID)
	if err != nil {
		ms.logger.Err(err)
		return
	}
	defer stream.Close()

	raw, err := json.Marshal(peerExchangeMessage{
		Id:             ms.Id(),
		Address:        ms.me,
		ExpectResponse: expectResponse,
	})
	if err != nil {
		ms.logger.Err(err)
		return
	}

	writer := bufio.NewWriter(stream)
	// We don't care about the number of bytes written
	_, err = writer.WriteString(string(raw) + string(DELIMITER))
	if err != nil {
		ms.logger.Err(err)
		return
	}
	writer.Flush()
}

// receivePeerInfo receives peer info from the given stream
func (ms *P2PMessageService) receivePeerInfo(stream network.Stream) {
	ms.logger.Debug().Msgf("received peerInfo")
	defer stream.Close()

	// Create a buffer stream for non blocking read and write.
	reader := bufio.NewReader(stream)
	raw, err := reader.ReadString(DELIMITER)

	// An EOF means the stream has been closed by the other side.
	if errors.Is(err, io.EOF) {
		return
	}
	if err != nil {
		ms.logger.Err(err)
		return
	}

	var msg *peerExchangeMessage
	err = json.Unmarshal([]byte(raw), &msg)
	if err != nil {
		ms.logger.Err(err)
		return
	}

	_, foundPeer := ms.peers.LoadOrStore(msg.Address.String(), msg.Id)
	if !foundPeer {
		peerInfo := basicPeerInfo{msg.Id, msg.Address}
		ms.logger.Debug().Msgf("stored new peer in map: %v", peerInfo)
		ms.newPeerInfo <- peerInfo
	}

	if msg.ExpectResponse {
		ms.sendPeerInfo(msg.Id, false)
	}
}

func (ms *P2PMessageService) getPeerIdFromDht(scaddr string) (peer.ID, error) {
	ms.logger.Info().Msgf("did not find address %s in local map, will query dht", scaddr)
	recordBytes, err := ms.dht.GetValue(context.Background(), DHT_RECORD_PREFIX+scaddr)
	if err != nil {
		return "", err
	}

	recordData := &dhtRecord{}
	err = json.Unmarshal(recordBytes, recordData)
	if err != nil {
		return "", err
	}

	peerId, err := peer.Decode(recordData.Data.PeerID)
	if err != nil {
		return "", err
	}
	ms.logger.Info().Msgf("found address in dht: %s (peerId: %s)", scaddr, peerId.String())

	ms.peers.Store(scaddr, peerId) // Cache this info locally for use next time
	return peerId, nil
}

// Send sends messages to other participants.
// It blocks until the message is sent.
// It will retry establishing a stream NUM_CONNECT_ATTEMPTS times before giving up
func (ms *P2PMessageService) Send(msg protocols.Message) error {
	raw, err := msg.Serialize()
	if err != nil {
		return err
	}

	// First try to get peerId from local "peers" map. If the address is not found there,
	// query the dht to retrieve the peerId, then store in local map for next time
	peerId, ok := ms.peers.Load(msg.To.String())
	if !ok {
		peerId, err = ms.getPeerIdFromDht(msg.To.String())
	}
	if err != nil {
		return err
	}

	for i := 0; i < NUM_CONNECT_ATTEMPTS; i++ {
		s, err := ms.p2pHost.NewStream(context.Background(), peerId, PROTOCOL_ID)
		if err == nil {
			writer := bufio.NewWriter(s)
			_, err = writer.WriteString(raw + string(DELIMITER)) // We don't care about the number of bytes written
			if err != nil {
				return err
			}

			writer.Flush()
			s.Close()
			return nil
		}

		ms.logger.Info().Int("attempt", i).Str("to", msg.To.String()).Msg("could not open stream: " + err.Error())
		time.Sleep(RETRY_SLEEP_DURATION)
	}
	return nil
}

// Out returns a channel that can be used to receive messages from the message service
func (ms *P2PMessageService) Out() <-chan protocols.Message {
	return ms.toEngine
}

// Close closes the P2PMessageService
func (ms *P2PMessageService) Close() error {
	// The mdns service is optional so we only close it if it exists
	if ms.mdns != nil {
		err := ms.mdns.Close()
		if err != nil {
			return err
		}
	}
	ms.p2pHost.RemoveStreamHandler(PROTOCOL_ID)
	return ms.p2pHost.Close()
}

// PeerInfoReceived returns a channel that receives a PeerInfo when a peer is discovered
func (ms *P2PMessageService) PeerInfoReceived() <-chan basicPeerInfo {
	return ms.newPeerInfo
}

func (ms *P2PMessageService) addBootPeers(peers []string) {
	for _, p := range peers {
		err := ms.addBootPeer(p)
		if err != nil {
			ms.logger.Info().Err(err)
		}
	}
}

func (ms *P2PMessageService) addBootPeer(p string) error {
	addr, err := multiaddr.NewMultiaddr(p)
	if err != nil {
		return err
	}

	peer, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return err
	}

	err = ms.p2pHost.Connect(context.Background(), *peer) // Adds peerInfo to local Peerstore
	if err != nil {
		return err
	}

	ms.logger.Debug().Msgf("connected to boot peer: %v", p)
	return nil
}
