//SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.10;

import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/utils/Address.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "./CosmosToken.sol";


contract GravityERC721 is ReentrancyGuard {
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
		address[] memory _gravitySolAddress,
	) {
		state_gravitySolAddress = _gravitySolAddress;
	}

	function sendERC721ToCosmos(
		address _tokenContract,
		string calldata _destination,
		uint256 _tokenId
	) external nonReentrant {

		///
		/// check is owner erc 721 goes here
		///

		state_lastERC721EventNonce = state_lastERC721EventNonce + 1;

		emit SendERC721ToCosmosEvent(
			_tokenContract,
			msg.sender,
			_destination,
			_tokenId, 
			state_lastERC721EventNonce
		);
	}


contract Gravity {
	bytes32 public state_lastValsetCheckpoint;
	uint256 public state_lastValsetNonce = 0;
	uint256 public state_lastEventNonce = 1;
	bytes32 public immutable state_gravityId;
}