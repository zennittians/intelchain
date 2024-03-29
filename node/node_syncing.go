package node

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/zennittians/intelchain/api/service/syncing"
	"github.com/zennittians/intelchain/api/service/syncing/downloader"
	downloader_pb "github.com/zennittians/intelchain/api/service/syncing/downloader/proto"
	"github.com/zennittians/intelchain/core"
	"github.com/zennittians/intelchain/core/types"
	nodeconfig "github.com/zennittians/intelchain/internal/configs/node"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/node/worker"
	"github.com/zennittians/intelchain/p2p"
)

// Constants related to doing syncing.
const (
	SyncFrequency = 60
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func init() {
	rand.Seed(time.Now().UnixNano())
}

// GenerateRandomString generates a random string with given length
func GenerateRandomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// getNeighborPeers is a helper function to return list of peers
// based on different neightbor map
func getNeighborPeers(neighbor *sync.Map) []p2p.Peer {
	tmp := []p2p.Peer{}
	neighbor.Range(func(k, v interface{}) bool {
		p := v.(p2p.Peer)
		t := p.Port
		p.Port = syncing.GetSyncingPort(t)
		tmp = append(tmp, p)
		return true
	})
	return tmp
}

// DoSyncWithoutConsensus gets sync-ed to blockchain without joining consensus
func (node *Node) DoSyncWithoutConsensus() {
	go node.DoSyncing(node.Blockchain(), node.Worker, false) //Don't join consensus
}

// IsSameHeight tells whether node is at same bc height as a peer
func (node *Node) IsSameHeight() (uint64, bool) {
	if node.stateSync == nil {
		node.stateSync = node.getStateSync()
	}
	return node.stateSync.IsSameBlockchainHeight(node.Blockchain())
}

func (node *Node) getStateSync() *syncing.StateSync {
	return syncing.CreateStateSync(node.SelfPeer.IP, node.SelfPeer.Port,
		node.GetSyncID(), node.NodeConfig.Role() == nodeconfig.ExplorerNode)
}

// SyncingPeerProvider is an interface for getting the peers in the given shard.
type SyncingPeerProvider interface {
	SyncingPeers(shardID uint32) (peers []p2p.Peer, err error)
}

// LegacySyncingPeerProvider uses neighbor lists stored in a Node to serve
// syncing peer list query.
type LegacySyncingPeerProvider struct {
	node    *Node
	shardID func() uint32
}

// NewLegacySyncingPeerProvider creates and returns a new node-based syncing
// peer provider.
func NewLegacySyncingPeerProvider(node *Node) *LegacySyncingPeerProvider {
	var shardID func() uint32
	if node.shardChains != nil {
		shardID = node.Blockchain().ShardID
	}
	return &LegacySyncingPeerProvider{node: node, shardID: shardID}
}

// SyncingPeers returns peers stored in neighbor maps in the node structure.
func (p *LegacySyncingPeerProvider) SyncingPeers(shardID uint32) (peers []p2p.Peer, err error) {
	switch shardID {
	case p.shardID():
		peers = getNeighborPeers(&p.node.Neighbors)
	case 0:
		peers = getNeighborPeers(&p.node.BeaconNeighbors)
	default:
		return nil, errors.Errorf("unsupported shard ID %v", shardID)
	}
	return peers, nil
}

// DNSSyncingPeerProvider uses the given DNS zone to resolve syncing peers.
type DNSSyncingPeerProvider struct {
	zone, port string
	lookupHost func(name string) (addrs []string, err error)
}

// NewDNSSyncingPeerProvider returns a provider that uses given DNS name and
// port number to resolve syncing peers.
func NewDNSSyncingPeerProvider(zone, port string) *DNSSyncingPeerProvider {
	return &DNSSyncingPeerProvider{
		zone:       zone,
		port:       port,
		lookupHost: net.LookupHost,
	}
}

// SyncingPeers resolves DNS name into peers and returns them.
func (p *DNSSyncingPeerProvider) SyncingPeers(shardID uint32) (peers []p2p.Peer, err error) {
	dns := fmt.Sprintf("s%d.%s", shardID, p.zone)
	addrs, err := p.lookupHost(dns)
	if err != nil {
		return nil, errors.Wrapf(err,
			"[SYNC] cannot find peers using DNS name %#v", dns)
	}
	for _, addr := range addrs {
		peers = append(peers, p2p.Peer{IP: addr, Port: p.port})
	}
	return peers, nil
}

// LocalSyncingPeerProvider uses localnet deployment convention to synthesize
// syncing peers.
type LocalSyncingPeerProvider struct {
	basePort, selfPort   uint16
	numShards, shardSize uint32
}

// NewLocalSyncingPeerProvider returns a provider that synthesizes syncing
// peers given the network configuration
func NewLocalSyncingPeerProvider(
	basePort, selfPort uint16, numShards, shardSize uint32,
) *LocalSyncingPeerProvider {
	return &LocalSyncingPeerProvider{
		basePort:  basePort,
		selfPort:  selfPort,
		numShards: numShards,
		shardSize: shardSize,
	}
}

// SyncingPeers returns local syncing peers using the sharding configuration.
func (p *LocalSyncingPeerProvider) SyncingPeers(shardID uint32) (peers []p2p.Peer, err error) {
	if shardID >= p.numShards {
		return nil, errors.Errorf(
			"shard ID %d out of range 0..%d", shardID, p.numShards-1)
	}
	firstPort := uint32(p.basePort) + shardID
	endPort := uint32(p.basePort) + p.numShards*p.shardSize
	for port := firstPort; port < endPort; port += p.numShards {
		if port == uint32(p.selfPort) {
			continue // do not sync from self
		}
		peers = append(peers, p2p.Peer{IP: "127.0.0.1", Port: fmt.Sprint(port)})
	}
	return peers, nil
}

// DoBeaconSyncing update received beaconchain blocks and downloads missing beacon chain blocks
func (node *Node) DoBeaconSyncing() {
	if node.NodeConfig.IsOffline {
		return
	}

	go func(node *Node) {
		// TODO ek – infinite loop; add shutdown/cleanup logic
		for beaconBlock := range node.BeaconBlockChannel {
			if node.beaconSync != nil {
				err := node.beaconSync.UpdateBlockAndStatus(
					beaconBlock, node.Beaconchain(), node.BeaconWorker, true,
				)
				if err != nil {
					node.beaconSync.AddLastMileBlock(beaconBlock)
				} else if node.Consensus.IsLeader() {
					// Only leader broadcast crosslink to avoid spamming p2p
					node.BroadcastCrossLink()
				}
			}
		}
	}(node)

	// TODO ek – infinite loop; add shutdown/cleanup logic
	for {
		if node.beaconSync == nil {
			utils.Logger().Info().Msg("initializing beacon sync")
			node.beaconSync = node.getStateSync()
		}
		if node.beaconSync.GetActivePeerNumber() == 0 {
			utils.Logger().Info().Msg("no peers; bootstrapping beacon sync config")
			// 0 means shardID=0 here
			peers, err := node.SyncingPeerProvider.SyncingPeers(0)
			if err != nil {
				utils.Logger().Warn().
					Err(err).
					Msg("cannot retrieve beacon syncing peers")
				continue
			}
			if err := node.beaconSync.CreateSyncConfig(peers, true); err != nil {
				utils.Logger().Warn().Err(err).Msg("cannot create beacon sync config")
				continue
			}
		}
		node.beaconSync.SyncLoop(node.Beaconchain(), node.BeaconWorker, true, nil)
		time.Sleep(time.Duration(SyncFrequency) * time.Second)
	}
}

// DoSyncing keep the node in sync with other peers, willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) DoSyncing(bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool) {
	if node.NodeConfig.IsOffline {
		return
	}

	ticker := time.NewTicker(time.Duration(SyncFrequency) * time.Second)
	// TODO ek – infinite loop; add shutdown/cleanup logic
	for {
		select {
		case <-ticker.C:
			node.doSync(bc, worker, willJoinConsensus)
		case <-node.Consensus.BlockNumLowChan:
			node.doSync(bc, worker, willJoinConsensus)
		}
	}
}

// doSync keep the node in sync with other peers, willJoinConsensus means the node will try to join consensus after catch up
func (node *Node) doSync(bc *core.BlockChain, worker *worker.Worker, willJoinConsensus bool) {
	if node.stateSync.GetActivePeerNumber() < syncing.NumPeersLowBound {
		shardID := bc.ShardID()
		peers, err := node.SyncingPeerProvider.SyncingPeers(shardID)
		if err != nil {
			utils.Logger().Warn().
				Err(err).
				Uint32("shard_id", shardID).
				Msg("cannot retrieve syncing peers")
			return
		}
		if err := node.stateSync.CreateSyncConfig(peers, false); err != nil {
			utils.Logger().Warn().
				Err(err).
				Interface("peers", peers).
				Msg("[SYNC] create peers error")
			return
		}
		utils.Logger().Debug().Int("len", node.stateSync.GetActivePeerNumber()).Msg("[SYNC] Get Active Peers")
	}
	// TODO: treat fake maximum height
	if node.stateSync.IsOutOfSync(bc, true) {
		node.IsInSync.UnSet()
		if willJoinConsensus {
			node.Consensus.BlocksNotSynchronized()
		}
		node.stateSync.SyncLoop(bc, worker, false, node.Consensus)
		if willJoinConsensus {
			node.IsInSync.Set()
			node.Consensus.BlocksSynchronized()
		}
	}
	node.IsInSync.Set()
}

// SupportBeaconSyncing sync with beacon chain for archival node in beacon chan or non-beacon node
func (node *Node) SupportBeaconSyncing() {
	go node.DoBeaconSyncing()
}

// SupportSyncing keeps sleeping until it's doing consensus or it's a leader.
func (node *Node) SupportSyncing() {
	node.InitSyncingServer()
	node.StartSyncingServer()

	joinConsensus := false
	// Check if the current node is explorer node.
	switch node.NodeConfig.Role() {
	case nodeconfig.Validator:
		joinConsensus = true
	}

	// Send new block to unsync node if the current node is not explorer node.
	// TODO: leo this pushing logic has to be removed
	if joinConsensus {
		go node.SendNewBlockToUnsync()
	}

	if node.stateSync == nil {
		node.stateSync = node.getStateSync()
		utils.Logger().Debug().Msg("[SYNC] initialized state sync")
	}

	go node.DoSyncing(node.Blockchain(), node.Worker, joinConsensus)
}

// InitSyncingServer starts downloader server.
func (node *Node) InitSyncingServer() {
	if node.downloaderServer == nil {
		node.downloaderServer = downloader.NewServer(node)
	}
}

// StartSyncingServer starts syncing server.
func (node *Node) StartSyncingServer() {
	utils.Logger().Info().Msg("[SYNC] support_syncing: StartSyncingServer")
	if node.downloaderServer.GrpcServer == nil {
		node.downloaderServer.Start(node.SelfPeer.IP, syncing.GetSyncingPort(node.SelfPeer.Port))
	}
}

// SendNewBlockToUnsync send latest verified block to unsync, registered nodes
func (node *Node) SendNewBlockToUnsync() {
	for {
		block := <-node.Consensus.VerifiedNewBlock
		blockHash, err := rlp.EncodeToBytes(block)
		if err != nil {
			utils.Logger().Warn().Msg("[SYNC] unable to encode block to hashes")
			continue
		}

		node.stateMutex.Lock()
		for peerID, config := range node.peerRegistrationRecord {
			elapseTime := time.Now().UnixNano() - config.timestamp
			if elapseTime > broadcastTimeout {
				utils.Logger().Warn().Str("peerID", peerID).Msg("[SYNC] SendNewBlockToUnsync to peer timeout")
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
				continue
			}
			response, err := config.client.PushNewBlock(node.GetSyncID(), blockHash, false)
			// close the connection if cannot push new block to unsync node
			if err != nil {
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
			}
			if response != nil && response.Type == downloader_pb.DownloaderResponse_INSYNC {
				node.peerRegistrationRecord[peerID].client.Close()
				delete(node.peerRegistrationRecord, peerID)
			}
		}
		node.stateMutex.Unlock()
	}
}

// CalculateResponse implements DownloadInterface on Node object.
func (node *Node) CalculateResponse(request *downloader_pb.DownloaderRequest, incomingPeer string) (*downloader_pb.DownloaderResponse, error) {
	response := &downloader_pb.DownloaderResponse{}
	if node.NodeConfig.IsOffline {
		return response, nil
	}

	switch request.Type {
	case downloader_pb.DownloaderRequest_BLOCKHASH:
		if request.BlockHash == nil {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request BlockHash is NIL")
		}
		if request.Size == 0 || request.Size > syncing.SyncLoopBatchSize {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request contains invalid Size %v", request.Size)
		}
		size := uint64(request.Size)
		var startHashHeader common.Hash
		copy(startHashHeader[:], request.BlockHash[:])
		startHeader := node.Blockchain().GetHeaderByHash(startHashHeader)
		if startHeader == nil {
			return response, fmt.Errorf("[SYNC] GetBlockHashes Request cannot find startHash %s", startHashHeader.Hex())
		}
		startHeight := startHeader.Number().Uint64()
		endHeight := node.Blockchain().CurrentBlock().NumberU64()
		if startHeight >= endHeight {
			utils.Logger().
				Debug().
				Uint64("myHeight", endHeight).
				Uint64("requestHeight", startHeight).
				Str("incomingIP", request.Ip).
				Str("incomingPort", request.Port).
				Str("incomingPeer", incomingPeer).
				Msg("[SYNC] GetBlockHashes Request: I am not higher than requested node")
			return response, nil
		}

		for blockNum := startHeight; blockNum <= startHeight+size; blockNum++ {
			header := node.Blockchain().GetHeaderByNumber(blockNum)
			if header == nil {
				break
			}
			blockHash := header.Hash()
			response.Payload = append(response.Payload, blockHash[:])
		}

	case downloader_pb.DownloaderRequest_BLOCKHEADER:
		var hash common.Hash
		for _, bytes := range request.Hashes {
			hash.SetBytes(bytes)
			encodedBlockHeader, err := node.getEncodedBlockHeaderByHash(hash)

			if err == nil {
				response.Payload = append(response.Payload, encodedBlockHeader)
			}
		}

	case downloader_pb.DownloaderRequest_BLOCK:
		var hash common.Hash
		for _, bytes := range request.Hashes {
			hash.SetBytes(bytes)
			encodedBlock, err := node.getEncodedBlockByHash(hash)

			if err == nil {
				response.Payload = append(response.Payload, encodedBlock)
			}
		}

	case downloader_pb.DownloaderRequest_BLOCKHEIGHT:
		response.BlockHeight = node.Blockchain().CurrentBlock().NumberU64()

	// this is the out of sync node acts as grpc server side
	case downloader_pb.DownloaderRequest_NEWBLOCK:
		if node.IsInSync.IsSet() {
			response.Type = downloader_pb.DownloaderResponse_INSYNC
			return response, nil
		}
		var blockObj types.Block
		err := rlp.DecodeBytes(request.BlockHash, &blockObj)
		if err != nil {
			utils.Logger().Warn().Msg("[SYNC] unable to decode received new block")
			return response, err
		}
		node.stateSync.AddNewBlock(request.PeerHash, &blockObj)

	case downloader_pb.DownloaderRequest_REGISTER:
		peerID := string(request.PeerHash[:])
		ip := request.Ip
		port := request.Port
		node.stateMutex.Lock()
		defer node.stateMutex.Unlock()
		if _, ok := node.peerRegistrationRecord[peerID]; ok {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.Logger().Warn().
				Interface("ip", ip).
				Interface("port", port).
				Msg("[SYNC] peerRegistration record already exists")
			return response, nil
		} else if len(node.peerRegistrationRecord) >= maxBroadcastNodes {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			utils.Logger().Debug().
				Str("ip", ip).
				Str("port", port).
				Msg("[SYNC] maximum registration limit exceeds")
			return response, nil
		} else {
			response.Type = downloader_pb.DownloaderResponse_FAIL
			syncPort := syncing.GetSyncingPort(port)
			client := downloader.ClientSetup(ip, syncPort)
			if client == nil {
				utils.Logger().Warn().
					Str("ip", ip).
					Str("port", port).
					Msg("[SYNC] unable to setup client for peerID")
				return response, nil
			}
			config := &syncConfig{timestamp: time.Now().UnixNano(), client: client}
			node.peerRegistrationRecord[peerID] = config
			utils.Logger().Debug().
				Str("ip", ip).
				Str("port", port).
				Msg("[SYNC] register peerID success")
			response.Type = downloader_pb.DownloaderResponse_SUCCESS
		}

	case downloader_pb.DownloaderRequest_REGISTERTIMEOUT:
		if !node.IsInSync.IsSet() {
			count := node.stateSync.RegisterNodeInfo()
			utils.Logger().Debug().
				Int("number", count).
				Msg("[SYNC] extra node registered")
		}
	}
	return response, nil
}

const (
	headerCacheSize = 10000
	blockCacheSize  = 10000
)

var (
	// Cached fields for block header and block requests
	headerReqCache, _ = lru.New(headerCacheSize)
	blockReqCache, _  = lru.New(blockCacheSize)

	errHeaderNotExist = errors.New("header not exist")
	errBlockNotExist  = errors.New("block not exist")
)

func (node *Node) getEncodedBlockHeaderByHash(hash common.Hash) ([]byte, error) {
	if b, ok := headerReqCache.Get(hash); ok {
		return b.([]byte), nil
	}
	h := node.Blockchain().GetHeaderByHash(hash)
	if h == nil {
		return nil, errHeaderNotExist
	}
	b, err := rlp.EncodeToBytes(h)
	if err != nil {
		return nil, err
	}
	headerReqCache.Add(hash, b)
	return b, nil
}

func (node *Node) getEncodedBlockByHash(hash common.Hash) ([]byte, error) {
	if b, ok := blockReqCache.Get(hash); ok {
		return b.([]byte), nil
	}
	blk := node.Blockchain().GetBlockByHash(hash)
	if blk == nil {
		return nil, errBlockNotExist
	}
	b, err := rlp.EncodeToBytes(blk)
	if err != nil {
		return nil, err
	}
	blockReqCache.Add(hash, b)
	return b, nil
}

// GetMaxPeerHeight ...
func (node *Node) GetMaxPeerHeight() uint64 {
	return node.stateSync.GetMaxPeerHeight()
}

// IsOutOfSync ...
func (node *Node) IsOutOfSync(bc *core.BlockChain) bool {
	return node.stateSync.IsOutOfSync(bc, false)
}
