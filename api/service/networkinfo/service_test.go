package networkinfo

import (
	"testing"
	"time"

	"github.com/zennittians/intelchain/crypto/bls"
	nodeconfig "github.com/zennittians/intelchain/internal/configs/node"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/p2p"
)

func TestService(t *testing.T) {
	nodePriKey, _, err := utils.LoadKeyFromFile("/tmp/127.0.0.1.12345.key")
	if err != nil {
		t.Fatal(err)
	}
	peerPriKey := bls.RandPrivateKey()
	peerPubKey := peerPriKey.GetPublicKey()
	if peerPriKey == nil || peerPubKey == nil {
		t.Fatal("generate key error")
	}
	selfPeer := p2p.Peer{IP: "127.0.0.1", Port: "12345", ConsensusPubKey: peerPubKey}
	host, err := p2p.NewHost(&selfPeer, nodePriKey)
	if err != nil {
		t.Fatal("unable to new host in intelchain")
	}

	s, err := New(host, nodeconfig.GroupIDBeaconClient, nil, nil, "")
	if err != nil {
		t.Fatalf("New() failed: %s", err)
	}

	s.StartService()
	time.Sleep(2 * time.Second)
	s.StopService()
}
