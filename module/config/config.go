package config

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// bech32PrefixAccAddr defines the bech32 prefix of an account's address
	bech32PrefixAccAddr = "gravity"
	// bech32PrefixAccPub defines the bech32 prefix of an account's public key
	bech32PrefixAccPub = "gravitypub"
	// bech32PrefixValAddr defines the bech32 prefix of a validator's operator address
	bech32PrefixValAddr = "gravityvaloper"
	// bech32PrefixValPub defines the bech32 prefix of a validator's operator public key
	bech32PrefixValPub = "gravityvaloperpub"
	// bech32PrefixConsAddr defines the bech32 prefix of a consensus node address
	bech32PrefixConsAddr = "gravityvalcons"
	// bech32PrefixConsPub defines the bech32 prefix of a consensus node public key
	bech32PrefixConsPub = "gravityvalconspub"

	// When accepting EIP-712 signed transactions, Gravity needs some sort of EVM ChainID.
	// Ethermint chains are forced to have a Cosmos Chain ID pattern like "gravity_1234-1",
	// where 1234 is the EVM Chain ID. It would be best to avoid changing Gravity's Chain ID so
	// this value is used in place of the restrictive Chain ID format.
	//
	// Note that this value is only usable with the github.com/althea-net fork of ethermint.
	// EIP-712 transactions are expected to use this Chain ID as the EIP712 Domain's Chain ID, and the
	// Cosmos Chain ID (e.g. "gravity-bridge-3") as the EIP712 Tx's Chain ID. Since both the Cosmos
	// and EVM Chain IDs are required the chance of replay attacks on other chains is very low,
	// but ensuring this is a unique value is good practice.
	GravityEvmChainID = "999999"

	// The native token, useful in situations where we do not have access to the sdk Context
	NativeTokenDenom = "ugraviton"
)

func init() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(bech32PrefixAccAddr, bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(bech32PrefixValAddr, bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(bech32PrefixConsAddr, bech32PrefixConsPub)
	config.Seal()
}
