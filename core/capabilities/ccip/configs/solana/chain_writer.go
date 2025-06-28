package solana

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"

	"github.com/smartcontractkit/chainlink-common/pkg/codec"
	types "github.com/smartcontractkit/chainlink-common/pkg/types/solana"

	idl "github.com/smartcontractkit/chainlink-ccip/chains/solana"
	ccipconsts "github.com/smartcontractkit/chainlink-ccip/pkg/consts"
)

var ccipOfframpIDL = idl.FetchCCIPOfframpIDL()
var ccipRouterIDL = idl.FetchCCIPRouterIDL()
var ccipCommonIDL = idl.FetchCommonIDL()

const (
	sourceChainSelectorPath       = "Info.AbstractReports.Messages.Header.SourceChainSelector"
	destTokenAddress              = "Info.AbstractReports.Messages.TokenAmounts.DestTokenAddress"
	tokenReceiverAddress          = "ExtraData.ExtraArgsDecoded.tokenReceiver"
	merkleRootSourceChainSelector = "Info.MerkleRoots.ChainSel"
	merkleRoot                    = "Info.MerkleRoots.MerkleRoot"
)

func getCommitMethodConfig(fromAddress string, offrampProgramAddress string, priceOnly bool) types.MethodConfig {
	chainSpecificName := "commit"
	if priceOnly {
		chainSpecificName = "commitPriceOnly"
	}
	return types.MethodConfig{
		FromAddress: fromAddress,
		InputModifications: []codec.ModifierConfig{
			&codec.RenameModifierConfig{
				Fields: map[string]string{"ReportContextByteWords": "ReportContext"},
			},
			&codec.RenameModifierConfig{
				Fields: map[string]string{"RawReport": "Report"},
			},
		},
		ChainSpecificName: chainSpecificName,
		ArgsTransform:     "CCIPCommit",
		LookupTables: types.LookupTables{
			DerivedLookupTables: []types.DerivedLookupTable{
				getCommonAddressLookupTableConfig(offrampProgramAddress),
			},
		},
		Accounts:        buildCommitAccountsList(fromAddress, offrampProgramAddress, priceOnly),
		DebugIDLocation: "",
	}
}

func buildCommitAccountsList(fromAddress, offrampProgramAddress string, priceOnly bool) []types.Lookup {
	accounts := []types.Lookup{}
	accounts = append(accounts,
		getOfframpAccountConfig(offrampProgramAddress),
		getReferenceAddressesConfig(offrampProgramAddress),
	)
	if !priceOnly {
		accounts = append(accounts,
			types.Lookup{
				PDALookups: &types.PDALookups{
					Name:      "SourceChainState",
					PublicKey: getAddressConstant(offrampProgramAddress),
					Seeds: []types.Seed{
						{Static: []byte("source_chain_state")},
						{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: merkleRootSourceChainSelector}}},
					},
					IsSigner:   false,
					IsWritable: true,
				},
			},
			types.Lookup{
				PDALookups: &types.PDALookups{
					Name:      "CommitReport",
					PublicKey: getAddressConstant(offrampProgramAddress),
					Seeds: []types.Seed{
						{Static: []byte("commit_report")},
						{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: merkleRootSourceChainSelector}}},
						{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: merkleRoot}}},
					},
					IsSigner:   false,
					IsWritable: true,
				},
			},
		)
	}
	accounts = append(accounts,
		getAuthorityAccountConstant(fromAddress),
		getSystemProgramConstant(),
		getSysVarInstructionConstant(),
		getFeeBillingSignerConfig(offrampProgramAddress),
		getFeeQuoterProgramAccount(offrampProgramAddress),
		getFeeQuoterAllowedPriceUpdater(offrampProgramAddress),
		getFeeQuoterConfigLookup(offrampProgramAddress),
		getRMNRemoteProgramAccount(offrampProgramAddress),
		getRMNRemoteCursesLookup(offrampProgramAddress),
		getRMNRemoteConfigLookup(offrampProgramAddress),
		getGlobalStateConfig(offrampProgramAddress),
		getBillingTokenConfig(offrampProgramAddress),
		getChainConfigGasPriceConfig(offrampProgramAddress),
	)
	return accounts
}

func getExecuteMethodConfig(fromAddress string, offrampProgramAddress string) types.MethodConfig {
	return types.MethodConfig{
		FromAddress: fromAddress,
		InputModifications: []codec.ModifierConfig{
			&codec.RenameModifierConfig{
				Fields: map[string]string{"ReportContextByteWords": "ReportContext"},
			},
			&codec.RenameModifierConfig{
				Fields: map[string]string{"RawExecutionReport": "Report"},
			},
		},
		ChainSpecificName:        "execute",
		ArgsTransform:            "CCIPExecute",
		ComputeUnitLimitOverhead: 150_000,
		BufferPayloadMethod:      "CCIPExecutionReportBuffer",
		LookupTables: types.LookupTables{
			DerivedLookupTables: []types.DerivedLookupTable{
				{
					Name: "PoolLookupTable",
					Accounts: types.Lookup{
						PDALookups: &types.PDALookups{
							Name:      "TokenAdminRegistry",
							PublicKey: getRouterProgramAccount(offrampProgramAddress),
							Seeds: []types.Seed{
								{Static: []byte("token_admin_registry")},
								{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: destTokenAddress}}},
							},
							IsSigner:   false,
							IsWritable: false,
							InternalField: types.InternalField{
								TypeName: "TokenAdminRegistry",
								Location: "LookupTable",
								// TokenAdminRegistry is in the common program so need to provide the IDL
								IDL: ccipCommonIDL,
							},
						},
					},
					Optional: true, // Lookup table is optional if DestTokenAddress is not present in report
				},
				getCommonAddressLookupTableConfig(offrampProgramAddress),
			},
		},
		ATAs: []types.ATALookup{
			{
				Location:      destTokenAddress,
				WalletAddress: types.Lookup{AccountLookup: &types.AccountLookup{Location: tokenReceiverAddress}},
				TokenProgram: types.Lookup{
					AccountsFromLookupTable: &types.AccountsFromLookupTable{
						LookupTableName: "PoolLookupTable",
						IncludeIndexes:  []int{6},
					},
				},
				MintAddress: types.Lookup{AccountLookup: &types.AccountLookup{Location: destTokenAddress}},
				Optional:    true, // ATA lookup is optional if DestTokenAddress is not present in report
			},
		},
		Accounts: []types.Lookup{
			getOfframpAccountConfig(offrampProgramAddress),
			getReferenceAddressesConfig(offrampProgramAddress),
			{
				PDALookups: &types.PDALookups{
					Name:      "SourceChainState",
					PublicKey: getAddressConstant(offrampProgramAddress),
					Seeds: []types.Seed{
						{Static: []byte("source_chain_state")},
						{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: sourceChainSelectorPath}}},
					},
					IsSigner:   false,
					IsWritable: false,
				},
			},
			{
				PDALookups: &types.PDALookups{
					Name:      "CommitReport",
					PublicKey: getAddressConstant(offrampProgramAddress),
					Seeds: []types.Seed{
						{Static: []byte("commit_report")},
						{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: sourceChainSelectorPath}}},
						{Dynamic: types.Lookup{
							AccountLookup: &types.AccountLookup{
								// The seed is the merkle root of the report, as passed into the input params.
								Location: merkleRoot,
							}},
						},
					},
					IsSigner:   false,
					IsWritable: true,
				},
			},
			getAddressConstant(offrampProgramAddress),
			{
				PDALookups: &types.PDALookups{
					Name:      "AllowedOfframp",
					PublicKey: getRouterProgramAccount(offrampProgramAddress),
					Seeds: []types.Seed{
						{Static: []byte("allowed_offramp")},
						{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: sourceChainSelectorPath}}},
						{Dynamic: getAddressConstant(offrampProgramAddress)},
					},
					IsSigner:   false,
					IsWritable: false,
				},
			},
			getAuthorityAccountConstant(fromAddress),
			getSystemProgramConstant(),
			getSysVarInstructionConstant(),
			getRMNRemoteProgramAccount(offrampProgramAddress),
			getRMNRemoteCursesLookup(offrampProgramAddress),
			getRMNRemoteConfigLookup(offrampProgramAddress),
			// logic receiver and user defined messaging accounts are appended in the CCIPExecute args transform
			// user token account, token billing config, pool chain config, and pool lookup table accounts
			// are appended to the accounts list in the CCIPExecute args transform for each token transfer
		},
		DebugIDLocation: "Info.AbstractReports.Messages.Header.MessageID",
	}
}

func GetSolanaChainWriterConfig(offrampProgramAddress string, fromAddress string) (types.ChainWriterConfig, error) {
	// check fromAddress
	pk, err := solana.PublicKeyFromBase58(fromAddress)
	if err != nil {
		return types.ChainWriterConfig{}, fmt.Errorf("invalid from address %s: %w", fromAddress, err)
	}

	if pk.IsZero() {
		return types.ChainWriterConfig{}, errors.New("from address cannot be empty")
	}

	// validate CCIP Offramp IDL, errors not expected
	var offrampIDL types.IDL
	if err = json.Unmarshal([]byte(ccipOfframpIDL), &offrampIDL); err != nil {
		return types.ChainWriterConfig{}, fmt.Errorf("unexpected error: invalid CCIP Offramp IDL, error: %w", err)
	}
	// validate CCIP Router IDL, errors not expected
	var routerIDL types.IDL
	if err = json.Unmarshal([]byte(ccipRouterIDL), &routerIDL); err != nil {
		return types.ChainWriterConfig{}, fmt.Errorf("unexpected error: invalid CCIP Router IDL, error: %w", err)
	}
	solConfig := types.ChainWriterConfig{
		Programs: map[string]types.ProgramConfig{
			ccipconsts.ContractNameOffRamp: {
				Methods: map[string]types.MethodConfig{
					ccipconsts.MethodExecute:         getExecuteMethodConfig(fromAddress, offrampProgramAddress),
					ccipconsts.MethodCommit:          getCommitMethodConfig(fromAddress, offrampProgramAddress, false),
					ccipconsts.MethodCommitPriceOnly: getCommitMethodConfig(fromAddress, offrampProgramAddress, true),
				},
				IDL: ccipOfframpIDL,
			},
		},
	}

	return solConfig, nil
}

func getOfframpAccountConfig(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name: "OfframpAccountConfig",
			PublicKey: types.Lookup{
				AccountConstant: &types.AccountConstant{
					Address: offrampProgramAddress,
				},
			},
			Seeds: []types.Seed{
				{Static: []byte("config")},
			},
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getAddressConstant(address string) types.Lookup {
	return types.Lookup{
		AccountConstant: &types.AccountConstant{
			Address:    address,
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getFeeQuoterProgramAccount(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      ccipconsts.ContractNameFeeQuoter,
			PublicKey: getAddressConstant(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("reference_addresses")},
			},
			IsSigner:   false,
			IsWritable: false,
			// Reads the address from the reference addresses account
			InternalField: types.InternalField{
				TypeName: "ReferenceAddresses",
				Location: "FeeQuoter",
				IDL:      ccipOfframpIDL,
			},
		},
	}
}

func getRouterProgramAccount(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      ccipconsts.ContractNameRouter,
			PublicKey: getAddressConstant(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("reference_addresses")},
			},
			IsSigner:   false,
			IsWritable: false,
			// Reads the address from the reference addresses account
			InternalField: types.InternalField{
				TypeName: "ReferenceAddresses",
				Location: "Router",
				IDL:      ccipOfframpIDL,
			},
		},
	}
}

func getReferenceAddressesConfig(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      "ReferenceAddresses",
			PublicKey: getAddressConstant(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("reference_addresses")},
			},
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getFeeBillingSignerConfig(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      "FeeBillingSigner",
			PublicKey: getAddressConstant(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("fee_billing_signer")},
			},
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getFeeQuoterAllowedPriceUpdater(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name: "FeeQuoterAllowedPriceUpdater",
			// Fetch fee quoter public key to use as program ID for PDA
			PublicKey: getFeeQuoterProgramAccount(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("allowed_price_updater")},
				{Dynamic: getFeeBillingSignerConfig(offrampProgramAddress)},
			},
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getFeeQuoterConfigLookup(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name: "FeeQuoterConfig",
			// Fetch fee quoter public key to use as program ID for PDA
			PublicKey: getFeeQuoterProgramAccount(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("config")},
			},
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getRMNRemoteProgramAccount(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      ccipconsts.ContractNameRMNRemote,
			PublicKey: getAddressConstant(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("reference_addresses")},
			},
			IsSigner:   false,
			IsWritable: false,
			// Reads the address from the reference addresses account
			InternalField: types.InternalField{
				TypeName: "ReferenceAddresses",
				Location: "RmnRemote",
				IDL:      ccipOfframpIDL,
			},
		},
	}
}

func getRMNRemoteCursesLookup(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      "RMNRemoteCurses",
			PublicKey: getRMNRemoteProgramAccount(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("curses")},
			},
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getRMNRemoteConfigLookup(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      "RMNRemoteConfig",
			PublicKey: getRMNRemoteProgramAccount(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("config")},
			},
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getGlobalStateConfig(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      "GlobalState",
			PublicKey: getAddressConstant(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("state")},
			},
			IsSigner:   false,
			IsWritable: true,
		},
		Optional: true,
	}
}

func getBillingTokenConfig(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      "BillingTokenConfig",
			PublicKey: getFeeQuoterProgramAccount(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("fee_billing_token_config")},
				{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: "Info.TokenPriceUpdates.TokenID"}}},
			},
			IsSigner:   false,
			IsWritable: true,
		},
		Optional: true,
	}
}

func getChainConfigGasPriceConfig(offrampProgramAddress string) types.Lookup {
	return types.Lookup{
		PDALookups: &types.PDALookups{
			Name:      "ChainConfigGasPrice",
			PublicKey: getFeeQuoterProgramAccount(offrampProgramAddress),
			Seeds: []types.Seed{
				{Static: []byte("dest_chain")},
				{Dynamic: types.Lookup{AccountLookup: &types.AccountLookup{Location: "Info.GasPriceUpdates.ChainSel"}}},
			},
			IsSigner:   false,
			IsWritable: true,
		},
		Optional: true,
	}
}

// getCommonAddressLookupTableConfig returns the lookup table config that fetches the lookup table address from a PDA on-chain
// The offramp contract contains a PDA with a ReferenceAddresses struct that stores the lookup table address in the OfframpLookupTable field
func getCommonAddressLookupTableConfig(offrampProgramAddress string) types.DerivedLookupTable {
	return types.DerivedLookupTable{
		Name: "CommonAddressLookupTable",
		Accounts: types.Lookup{
			PDALookups: &types.PDALookups{
				Name:      "OfframpLookupTable",
				PublicKey: getAddressConstant(offrampProgramAddress),
				Seeds: []types.Seed{
					{Static: []byte("reference_addresses")},
				},
				InternalField: types.InternalField{
					TypeName: "ReferenceAddresses",
					Location: "OfframpLookupTable",
					IDL:      ccipOfframpIDL,
				},
			},
		},
	}
}

func getAuthorityAccountConstant(fromAddress string) types.Lookup {
	return types.Lookup{
		AccountConstant: &types.AccountConstant{
			Name:       "Authority",
			Address:    fromAddress,
			IsSigner:   true,
			IsWritable: true,
		},
	}
}

func getSystemProgramConstant() types.Lookup {
	return types.Lookup{
		AccountConstant: &types.AccountConstant{
			Name:       "SystemProgram",
			Address:    solana.SystemProgramID.String(),
			IsSigner:   false,
			IsWritable: false,
		},
	}
}

func getSysVarInstructionConstant() types.Lookup {
	return types.Lookup{
		AccountConstant: &types.AccountConstant{
			Name:       "SysvarInstructions",
			Address:    solana.SysVarInstructionsPubkey.String(),
			IsSigner:   false,
			IsWritable: false,
		},
	}
}
