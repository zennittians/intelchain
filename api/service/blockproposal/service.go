package blockproposal

import (
	"github.com/ethereum/go-ethereum/rpc"
	msg_pb "github.com/zennittians/intelchain/api/proto/message"
	"github.com/zennittians/intelchain/consensus"
	"github.com/zennittians/intelchain/internal/utils"
)

// Service is a block proposal service.
type Service struct {
	stopChan              chan struct{}
	stoppedChan           chan struct{}
	readySignal           chan consensus.ProposalType
	commitSigsChan        chan []byte
	messageChan           chan *msg_pb.Message
	waitForConsensusReady func(readySignal chan consensus.ProposalType, commitSigsChan chan []byte, stopChan chan struct{}, stoppedChan chan struct{})
}

// New returns a block proposal service.
func New(readySignal chan consensus.ProposalType, commitSigsChan chan []byte, waitForConsensusReady func(readySignal chan consensus.ProposalType, commitSigsChan chan []byte, stopChan chan struct{}, stoppedChan chan struct{})) *Service {
	return &Service{readySignal: readySignal, commitSigsChan: commitSigsChan, waitForConsensusReady: waitForConsensusReady}
}

// StartService starts block proposal service.
func (s *Service) StartService() {
	s.stopChan = make(chan struct{})
	s.stoppedChan = make(chan struct{})

	s.Init()
	s.Run(s.stopChan, s.stoppedChan)
}

// Init initializes block proposal service.
func (s *Service) Init() {
}

// Run runs block proposal.
func (s *Service) Run(stopChan chan struct{}, stoppedChan chan struct{}) {
	s.waitForConsensusReady(s.readySignal, s.commitSigsChan, s.stopChan, s.stoppedChan)
}

// StopService stops block proposal service.
func (s *Service) StopService() {
	utils.Logger().Info().Msg("Stopping block proposal service.")
	s.stopChan <- struct{}{}
	<-s.stoppedChan
	utils.Logger().Info().Msg("Role conversion stopped.")
}

// NotifyService notify service
func (s *Service) NotifyService(params map[string]interface{}) {}

// SetMessageChan sets up message channel to service.
func (s *Service) SetMessageChan(messageChan chan *msg_pb.Message) {
	s.messageChan = messageChan
}

// APIs for the services.
func (s *Service) APIs() []rpc.API {
	return nil
}
