package types

import (
	"crypto/ecdsa"
	fmt "fmt"
	"math/big"

	errorsmod "cosmossdk.io/errors"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	signaturePrefix = "\x19Ethereum Signed Message:\n32"
)

// NewEthereumSignature creates a new signature over a given byte array
func NewEthereumSignature(hash []byte, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	if privateKey == nil {
		return nil, errorsmod.Wrap(ErrEmpty, "private key")
	}
	protectedHash := crypto.Keccak256Hash(append([]uint8(signaturePrefix), hash...))
	return crypto.Sign(protectedHash.Bytes(), privateKey)
}

// decodeSignature was duplicated from go-ethereum with slight modifications
func decodeSignature(sig []byte) (r, s *big.Int, v byte) {
	if len(sig) != crypto.SignatureLength {
		panic(fmt.Sprintf("wrong size for signature: got %d, want %d", len(sig), crypto.SignatureLength))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	if sig[64] == 27 || sig[64] == 28 {
		v = sig[64] - 27
	} else {
		v = sig[64]
	}
	return r, s, v
}

func EthAddressFromSignature(hash []byte, signature []byte) (*EthAddress, error) {
	if len(signature) < 65 {
		return nil, errorsmod.Wrap(ErrInvalid, "signature too short")
	}

	r, s, v := decodeSignature(signature)
	if !crypto.ValidateSignatureValues(v, r, s, true) {
		return nil, errorsmod.Wrap(ErrInvalid, "Signature values failed validation")
	}

	// To verify signature
	// - use crypto.SigToPub to get the public key
	// - use crypto.PubkeyToAddress to get the address
	// - compare this to the address given.

	// for backwards compatibility reasons  the V value of an Ethereum sig is presented
	// as 27 or 28, internally though it should be a 0-3 value due to changed formats.
	// It seems that go-ethereum expects this to be done before sigs actually reach it's
	// internal validation functions. In order to comply with this requirement we check
	// the sig an dif it's in standard format we correct it. If it's in go-ethereum's expected
	// format already we make no changes.
	//
	// We could attempt to break or otherwise exit early on obviously invalid values for this
	// byte, but that's a task best left to go-ethereum
	if signature[64] == 27 || signature[64] == 28 {
		signature[64] -= 27
	}

	protectedHash := crypto.Keccak256Hash(append([]uint8(signaturePrefix), hash...))

	pubkey, err := crypto.SigToPub(protectedHash.Bytes(), signature)
	if err != nil {
		return nil, errorsmod.Wrap(err, "signature to public key")
	}

	addr, err := NewEthAddress(crypto.PubkeyToAddress(*pubkey).Hex())
	if err != nil {
		return nil, errorsmod.Wrap(err, "invalid address from public key")
	}

	return addr, nil
}

// ValidateEthereumSignature takes a message, an associated signature and public key and
// returns an error if the signature isn't valid
func ValidateEthereumSignature(hash []byte, signature []byte, ethAddress EthAddress) error {
	addr, err := EthAddressFromSignature(hash, signature)

	if err != nil {
		return errorsmod.Wrap(err, "unable to get address from signature")
	}

	if addr.GetAddress() != ethAddress.GetAddress() {
		return errorsmod.Wrap(ErrInvalid, "signature not matching")
	}

	return nil
}
