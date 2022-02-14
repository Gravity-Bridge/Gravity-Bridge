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

error InvalidBatchNonce(uint256 newNonce, uint256 currentNonce);
error BatchTimedOut();
error IncorrectCheckpoint();
error InvalidLogicCallTransfers();
error MalformedBatch();

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

	event TxnERC721BatchExecutedEvent(
		uint256 indexed _batchNonce,
		address indexed _token,
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

	// submitBatch processes a batch of Cosmos -> Ethereum transactions by sending the tokens in the transactions
	// to the destination addresses. It is approved by the current Cosmos validator set.
	// Anyone can call this function, but they must supply valid signatures of constant_powerThreshold of the current valset over
	// the batch.
	function submitBatch(
		ValsetArgs calldata _currentValset,
		Signature[] calldata _sigs,
		uint256[] calldata _tokenIds,
		address[] calldata _destinations,
		uint256[] calldata _fees,
		uint256 _batchNonce,
		address _tokenContract,
		uint256 _batchTimeout
	) external nonReentrant {
		// CHECKS scoped to reduce stack depth
		{
			Gravity g = Gravity(_gravitySolAddress);

			if (_batchNonce <= g.state_lastBatchNonces[_tokenContract]) {
				revert InvalidBatchNonce({
					newNonce: _batchNonce,
					currentNonce: g.state_lastBatchNonces[_tokenContract]
				});
			}

			if (_batchNonce > g.state_lastBatchNonces[_tokenContract] + 1000000) {
				revert InvalidBatchNonce({
					newNonce: _batchNonce,
					currentNonce: state_lastBatchNonces[_tokenContract]
				});
			}

			if (g.block.number >= _batchTimeout) {
				revert BatchTimedOut();
			}

			g.validateValset(_currentValset, _sigs);

			if (g.makeCheckpoint(_currentValset, g.state_gravityId) != g.state_lastValsetCheckpoint) {
				revert IncorrectCheckpoint();
			}

			// Check that the transaction batch is well-formed
			if (_tokenIds.length != _destinations.length || _tokenIds.length != _fees.length) {
				revert MalformedBatch();
			}

			// Check that enough current validators have signed off on the transaction batch and valset
			g.checkValidatorSignatures(
				_currentValset,
				_sigs,
				// Get hash of the transaction batch and checkpoint
				keccak256(
					abi.encode(
						g.state_gravityId,
						// bytes32 encoding of "transactionBatch"
						0x7472616e73616374696f6e426174636800000000000000000000000000000000,
						_tokenIds,
						_destinations,
						_fees,
						_batchNonce,
						_tokenContract,
						_batchTimeout
					)
				),
				g.constant_powerThreshold
			);

			g.state_lastBatchNonces[_tokenContract] = _batchNonce;

			{
				// Send transaction amounts to destinations
				uint256 totalFee;
				for (uint256 i = 0; i < _tokenIds.length; i++) {
					ERC721(_tokenContract).safeTransferFrom(address(this), _destinations[i], _tokenIds[i]);
					totalFee = totalFee + _fees[i];
				}

				// Send transaction fees to msg.sender
				IERC20(_tokenContract).safeTransfer(msg.sender, totalFee);
			}
		}

		{
			state_lastERC721EventNonce = state_lastERC721EventNonce + 1;
			emit TxnERC721BatchExecutedEvent(_batchNonce, _tokenContract, state_lastERC721EventNonce);
		}
	}
}