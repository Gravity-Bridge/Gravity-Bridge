package config

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// Bech32PrefixAccAddr defines the bech32 prefix of an account's address
	Bech32PrefixAccAddr = "gravity"
	// Bech32PrefixAccPub defines the bech32 prefix of an account's public key
	Bech32PrefixAccPub = "gravitypub"
	// Bech32PrefixValAddr defines the bech32 prefix of a validator's operator address
	Bech32PrefixValAddr = "gravityvaloper"
	// Bech32PrefixValPub defines the bech32 prefix of a validator's operator public key
	Bech32PrefixValPub = "gravityvaloperpub"
	// Bech32PrefixConsAddr defines the bech32 prefix of a consensus node address
	Bech32PrefixConsAddr = "gravityvalcons"
	// Bech32PrefixConsPub defines the bech32 prefix of a consensus node public key
	Bech32PrefixConsPub = "gravityvalconspub"

	// The native token, useful in situations where we do not have access to the sdk Context
	NativeTokenDenom = "ugraviton"
)

var (
	// When accepting EIP-712 signed transactions, Gravity needs some sort of EVM ChainID.
	// Ethermint chains are forced to have a Cosmos Chain ID pattern like "gravity_1234-1",
	// where 1234 is the EVM Chain ID. It would be best to avoid changing Gravity's Chain ID so
	// these values are used in place of the restrictive Chain ID format.
	//
	// Note that these values are only usable with the github.com/althea-net fork of ethermint.
	// EIP-712 transactions are expected to use these Chain IDs as the EIP712 Domain's Chain ID, and the
	// Cosmos Chain ID (e.g. "gravity-bridge-3") as the EIP712 Tx's Chain ID. Since both the Cosmos
	// and EVM Chain IDs are required the chance of replay attacks on other chains is very low
	//
	// These are in order, the original Gravity EVM chain id, the new Gravity EVM chain id, the Canto chain id, and the Althea-L1 chain id
	GravityEvmChainIDs = []string{"999999", "180086", "7700", "258432"}
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
	config.Seal()
}
