//SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.10;

import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/utils/Address.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "./CosmosToken.sol";
import "./Gravity.sol";
import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import { ERC721Holder } from "@openzeppelin/contracts/token/ERC721/utils/ERC721Holder.sol";
import "hardhat/console.sol"; 


contract GravityERC721 is ERC721Holder, ReentrancyGuard {

	uint256 public state_lastERC721EventNonce = 1;
	address public state_gravitySolAddress;

	event SendERC721ToCosmosEvent(
		address indexed _tokenContract,
		address indexed _sender,
		string _destination,
		uint256 _tokenId,
		uint256 _eventNonce
	);

	constructor(
		// reference gravity.sol for many functions peformed here
		address _gravitySolAddress
	) {
		state_gravitySolAddress = _gravitySolAddress;
	}

	function sendERC721ToCosmos(
		address _tokenContract,
		string calldata _destination,
		uint256 _tokenId
	) external nonReentrant {

		ERC721(_tokenContract).safeTransferFrom(msg.sender, address(this), _tokenId);
		
		state_lastERC721EventNonce = state_lastERC721EventNonce + 1;

		emit SendERC721ToCosmosEvent(
			_tokenContract,
			msg.sender,
			_destination,
			_tokenId, 
			state_lastERC721EventNonce
		);
	}
}
