package ccipton

import (
	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/loop"
	"github.com/smartcontractkit/chainlink-common/pkg/types"
	"github.com/smartcontractkit/chainlink-common/pkg/types/ccipocr3"
)

// TONChainAccessorFactory implements cciptypes.ChainAccessorFactory for TON chains.
type TONChainAccessorFactory struct{}

// NewChainAccessor creates a new chain accessor to be used for TON chains.
func (f TONChainAccessorFactory) NewChainAccessor(
	lggr logger.Logger,
	relayer loop.Relayer,
	chainSelector ccipocr3.ChainSelector,
	_ types.ContractReader,
	_ types.ContractWriter,
	addrCodec ccipocr3.AddressCodec,
) (ccipocr3.ChainAccessor, error) {
	// TODO(NONEVM-1460): Implement TON chain accessor using TONService, or other smaller-scoped components
	//   like LogPollerLite from the relayer object.
	/**
	It will likely look something like this:
		ca, err := relayer.NewChainAccessor(ctx)
	*/
	panic("implement me")
}
