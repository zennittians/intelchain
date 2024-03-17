package worker

import "github.com/zennittians/intelchain/block"

type Environment interface {
	CurrentHeader() *block.Header
}
