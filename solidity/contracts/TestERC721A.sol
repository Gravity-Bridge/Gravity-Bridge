//SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.10;
import "@openzeppelin/contracts/token/ERC721/ERC721.sol";

// One of three testing coins
contract TestERC721A is ERC721 {
	constructor() ERC721("NFT PUNK", "NFTPUNK") {
		_mint(0xc783df8a850f42e7F7e57013759C285caa701eB6, 190);
		_mint(0xeAD9C93b79Ae7C1591b1FB5323BD777E86e150d4, 190);
		_mint(0xE5904695748fe4A84b40b3fc79De2277660BD1D3, 190);
		_mint(0x92561F28Ec438Ee9831D00D1D59fbDC981b762b2, 190);
		_mint(0x2fFd013AaA7B5a7DA93336C2251075202b33FB2B, 190);
		// this is the EtherBase address for our testnet miner in
		// tests/assets/ETHGenesis.json so it wil have both a lot
		// of ETH and a lot of erc20 tokens to test with
		_mint(0xBf660843528035a5A4921534E156a27e64B231fE, 190);
	}
}
