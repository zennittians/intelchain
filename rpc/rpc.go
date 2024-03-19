package rpc

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	nodeconfig "github.com/zennittians/intelchain/internal/configs/node"
	"github.com/zennittians/intelchain/internal/utils"
	"github.com/zennittians/intelchain/itc"
	eth "github.com/zennittians/intelchain/rpc/eth"
	v1 "github.com/zennittians/intelchain/rpc/v1"
	v2 "github.com/zennittians/intelchain/rpc/v2"
)

// Version enum
const (
	V1 Version = iota
	V2
	Eth
	Debug
)

const (
	// APIVersion used for DApp's, bumped after RPC refactor (7/2020)
	APIVersion = "1.1"
	// CallTimeout is the timeout given to all contract calls
	CallTimeout = 5 * time.Second
	// LogTag is the tag found in the log for all RPC logs
	LogTag = "[RPC]"
	// HTTPPortOffset ..
	HTTPPortOffset = 500
	// WSPortOffset ..
	WSPortOffset = 800

	netNamespace   = "net"
	netV1Namespace = "netv1"
	netV2Namespace = "netv2"
)

var (
	// HTTPModules ..
	HTTPModules = []string{"itc", "itcv2", "eth", "debug", netNamespace, netV1Namespace, netV2Namespace, "explorer"}
	// WSModules ..
	WSModules = []string{"itc", "itcv2", "eth", "debug", netNamespace, netV1Namespace, netV2Namespace, "web3"}

	httpListener     net.Listener
	httpHandler      *rpc.Server
	wsListener       net.Listener
	wsHandler        *rpc.Server
	httpEndpoint     = ""
	wsEndpoint       = ""
	httpVirtualHosts = []string{"*"}
	httpTimeouts     = rpc.DefaultHTTPTimeouts
	httpOrigins      = []string{"*"}
	wsOrigins        = []string{"*"}
)

// Version of the RPC
type Version int

// Namespace of the RPC version
func (n Version) Namespace() string {
	return HTTPModules[n]
}

// StartServers starts the http & ws servers
func StartServers(itc *itc.Intelchain, apis []rpc.API, config nodeconfig.RPCServerConfig) error {
	apis = append(apis, getAPIs(itc, config.DebugEnabled)...)

	if config.HTTPEnabled {
		httpEndpoint = fmt.Sprintf("%v:%v", config.HTTPIp, config.HTTPPort)
		if err := startHTTP(apis); err != nil {
			return err
		}
	}

	if config.WSEnabled {
		wsEndpoint = fmt.Sprintf("%v:%v", config.WSIp, config.WSPort)
		if err := startWS(apis); err != nil {
			return err
		}
	}

	return nil
}

// StopServers stops the http & ws servers
func StopServers() error {
	if httpListener != nil {
		if err := httpListener.Close(); err != nil {
			return err
		}
		httpListener = nil
		utils.Logger().Info().
			Str("url", fmt.Sprintf("http://%s", httpEndpoint)).
			Msg("HTTP endpoint closed")
	}
	if httpHandler != nil {
		httpHandler.Stop()
		httpHandler = nil
	}
	if wsListener != nil {
		if err := wsListener.Close(); err != nil {
			return err
		}
		wsListener = nil
		utils.Logger().Info().
			Str("url", fmt.Sprintf("http://%s", wsEndpoint)).
			Msg("WS endpoint closed")
	}
	if wsHandler != nil {
		wsHandler.Stop()
		wsHandler = nil
	}
	return nil
}

// getAPIs returns all the API methods for the RPC interface
func getAPIs(itc *itc.Intelchain, debugEnable bool) []rpc.API {
	publicAPIs := []rpc.API{
		// Public methods
		NewPublicIntelchainAPI(itc, V1),
		NewPublicIntelchainAPI(itc, V2),
		NewPublicIntelchainAPI(itc, Eth),
		NewPublicBlockchainAPI(itc, V1),
		NewPublicBlockchainAPI(itc, V2),
		NewPublicBlockchainAPI(itc, Eth),
		NewPublicContractAPI(itc, V1),
		NewPublicContractAPI(itc, V2),
		NewPublicContractAPI(itc, Eth),
		NewPublicTransactionAPI(itc, V1),
		NewPublicTransactionAPI(itc, V2),
		NewPublicTransactionAPI(itc, Eth),
		NewPublicPoolAPI(itc, V1),
		NewPublicPoolAPI(itc, V2),
		NewPublicPoolAPI(itc, Eth),
		NewPublicStakingAPI(itc, V1),
		NewPublicStakingAPI(itc, V2),
		NewPublicTracerAPI(itc, Debug),
		// Legacy methods (subject to removal)
		v1.NewPublicLegacyAPI(itc, "itc"),
		eth.NewPublicEthService(itc, "eth"),
		v2.NewPublicLegacyAPI(itc, "itcv2"),
	}

	privateAPIs := []rpc.API{
		NewPrivateDebugAPI(itc, V1),
		NewPrivateDebugAPI(itc, V2),
	}

	if debugEnable {
		return append(publicAPIs, privateAPIs...)
	}
	return publicAPIs
}

func startHTTP(apis []rpc.API) (err error) {
	httpListener, httpHandler, err = rpc.StartHTTPEndpoint(
		httpEndpoint, apis, HTTPModules, httpOrigins, httpVirtualHosts, httpTimeouts,
	)
	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("url", fmt.Sprintf("http://%s", httpEndpoint)).
		Str("cors", strings.Join(httpOrigins, ",")).
		Str("vhosts", strings.Join(httpVirtualHosts, ",")).
		Msg("HTTP endpoint opened")
	return nil
}

func startWS(apis []rpc.API) (err error) {
	wsListener, wsHandler, err = rpc.StartWSEndpoint(wsEndpoint, apis, WSModules, wsOrigins, true)
	if err != nil {
		return err
	}

	utils.Logger().Info().
		Str("url", fmt.Sprintf("ws://%s", wsListener.Addr())).
		Msg("WebSocket endpoint opened")
	return nil
}
