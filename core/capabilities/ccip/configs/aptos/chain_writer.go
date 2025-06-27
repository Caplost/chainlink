package aptosconfig

import (
	"fmt"

	"github.com/smartcontractkit/chainlink-aptos/relayer/utils"
	"github.com/smartcontractkit/chainlink-ccip/pkg/consts"
	"github.com/smartcontractkit/chainlink-common/pkg/types/aptos"
)

func GetChainWriterConfig(publicKeyStr string) (aptos.ChainWriterConfig, error) {
	fromAddress, err := utils.HexPublicKeyToAddress(publicKeyStr)
	if err != nil {
		return aptos.ChainWriterConfig{}, fmt.Errorf("failed to parse Aptos address from public key %s: %w", publicKeyStr, err)
	}

	return aptos.ChainWriterConfig{
		Modules: map[string]*aptos.ChainWriterModule{
			consts.ContractNameOffRamp: {
				Name: "offramp",
				Functions: map[string]*aptos.ChainWriterFunction{
					consts.MethodCommit: {
						Name:        "commit",
						PublicKey:   publicKeyStr,
						FromAddress: fromAddress.String(),
						Params: []aptos.FunctionParam{
							{
								Name:     "ReportContext",
								Type:     "vector<vector<u8>>",
								Required: true,
							},
							{
								Name:     "Report",
								Type:     "vector<u8>",
								Required: true,
							},
							{
								Name:     "Signatures",
								Type:     "vector<vector<u8>>",
								Required: true,
							},
						},
					},
					consts.MethodExecute: {
						Name:        "execute",
						PublicKey:   publicKeyStr,
						FromAddress: fromAddress.String(),
						Params: []aptos.FunctionParam{
							{
								Name:     "ReportContext",
								Type:     "vector<vector<u8>>",
								Required: true,
							},
							{
								Name:     "Report",
								Type:     "vector<u8>",
								Required: true,
							},
						},
					},
				},
			},
		},
		FeeStrategy: aptos.DefaultFeeStrategy,
	}, nil
}
