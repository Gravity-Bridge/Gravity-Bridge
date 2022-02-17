//SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.10;

import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/utils/Address.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "./CosmosToken.sol";
import "./Gravity.sol";
import "./GravityERC721Withdraw.sol"; 
import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import { ERC721Holder } from "@openzeppelin/contracts/token/ERC721/utils/ERC721Holder.sol";

struct tokenContractBatchAddrs {
	address tokenContractERC721;
	address tokenContractERC20;
}


contract GravityERC721 is ERC721Holder, ReentrancyGuard {
	
	uint256 public state_lastERC721EventNonce = 1;
	address public state_gravitySolAddress;
	mapping(address => uint256) public state_lastERC721BatchNonces;
	uint256 constant constant_powerThreshold = 2863311530;

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

	// function validateValset(ValsetArgs calldata _valset, Signature[] calldata _sigs) private pure {
	// 	if (
	// 		_valset.validators.length != _valset.powers.length ||
	// 		_valset.validators.length != _sigs.length
	// 	) {
	// 		revert MalformedCurrentValidatorSet();
	// 	}
	// }

	// function makeCheckpoint(ValsetArgs memory _valsetArgs, bytes32 _gravityId)
	// 	private
	// 	pure
	// 	returns (bytes32)
	// {
	// 	// bytes32 encoding of the string "checkpoint"
	// 	bytes32 methodName = 0x636865636b706f696e7400000000000000000000000000000000000000000000;

	// 	bytes32 checkpoint = keccak256(
	// 		abi.encode(
	// 			_gravityId,
	// 			methodName,
	// 			_valsetArgs.valsetNonce,
	// 			_valsetArgs.validators,
	// 			_valsetArgs.powers,
	// 			_valsetArgs.rewardAmount,
	// 			_valsetArgs.rewardToken
	// 		)
	// 	);
	// 	return checkpoint;
	// }

	// function checkValidatorSignatures(
	// 	ValsetArgs calldata _currentValset,
	// 	Signature[] calldata _sigs,
	// 	bytes32 _theHash,
	// 	uint256 _powerThreshold
	// ) private pure {
	// 	uint256 cumulativePower = 0;

	// 	for (uint256 i = 0; i < _currentValset.validators.length; i++) {
	// 		if (_sigs[i].v != 0) {
	// 			if (!verifySig(_currentValset.validators[i], _theHash, _sigs[i])) {
	// 				revert InvalidSignature();
	// 			}
	// 			cumulativePower = cumulativePower + _currentValset.powers[i];
	// 			if (cumulativePower > _powerThreshold) {
	// 				break;
	// 			}
	// 		}
	// 	}
	// 	if (cumulativePower <= _powerThreshold) {
	// 		revert InsufficientPower(cumulativePower, _powerThreshold);
	// 	}
	// }

	// function verifySig(
	// 	address _signer,
	// 	bytes32 _theHash,
	// 	Signature calldata _sig
	// ) private pure returns (bool) {
	// 	bytes32 messageDigest = keccak256(
	// 		abi.encodePacked("\x19Ethereum Signed Message:\n32", _theHash)
	// 	);
	// 	return _signer == ECDSA.recover(messageDigest, _sig.v, _sig.r, _sig.s);
	// }

	// function checkBatchValidation(
	// 	ValsetArgs calldata _currentValset,
	// 	Signature[] calldata _sigs,
	// 	uint256[] calldata _tokenIds,
	// 	address[] calldata _destinations,
	// 	uint256[] calldata _fees,
	// 	uint256 _batchNonce,
	// 	address _tokenContractERC721,
	// 	uint256 _batchTimeout
	// ) private  {
	// 	Gravity g = Gravity(state_gravitySolAddress);
	// 	if (_batchNonce <= g.state_lastBatchNonces(_tokenContractERC721)) {
	// 			revert InvalidBatchNonce({
	// 				newNonce: _batchNonce,
	// 				currentNonce: g.state_lastBatchNonces(_tokenContractERC721)
	// 			});
	// 		}

	// 		if (_batchNonce > g.state_lastBatchNonces(_tokenContractERC721) + 1000000) {
	// 			revert InvalidBatchNonce({
	// 				newNonce: _batchNonce,
	// 				currentNonce: g.state_lastBatchNonces(_tokenContractERC721)
	// 			});
	// 		}

	// 		if (block.number >= _batchTimeout) {
	// 			revert BatchTimedOut();
	// 		}

	// 		validateValset(_currentValset, _sigs);

	// 		if (makeCheckpoint(_currentValset, g.state_gravityId()) != g.state_lastValsetCheckpoint()) {
	// 			revert IncorrectCheckpoint();
	// 		}

	// 		// Check that the transaction batch is well-formed
	// 		if (_tokenIds.length != _destinations.length || _tokenIds.length != _fees.length) {
	// 			revert MalformedBatch();
	// 		}

	// 		checkValidatorSignatures(
	// 			_currentValset,
	// 			_sigs,
	// 			// Get hash of the transaction batch and checkpoint
	// 			keccak256(
	// 				abi.encode(
	// 					g.state_gravityId(),
	// 					// bytes32 encoding of "transactionBatch"
	// 					0x7472616e73616374696f6e426174636800000000000000000000000000000000,
	// 					_tokenIds,
	// 					_destinations,
	// 					_fees,
	// 					_batchNonce,
	// 					_tokenContractERC721,
	// 					_batchTimeout
	// 				)
	// 			),
	// 			constant_powerThreshold
	// 		);
	// }

	// }

	// function submitERC721Batch(
	// 	ValsetArgs calldata _currentValset,
	// 	Signature[] calldata _sigs,
	// 	uint256[] calldata _tokenIds,
	// 	address[] calldata _destinations,
	// 	uint256[] calldata _fees,
	// 	uint256 _batchNonce,
	// 	tokenContractBatchAddrs memory _tokenContracts,
	// 	uint256 _batchTimeout
	// ) external nonReentrant {
	// 	// CHECKS scoped to reduce stack depth
	// 	{
	// 		address _tokenContractERC721 = _tokenContracts.tokenContractERC721;
	// 		address _tokenContractERC20 = _tokenContracts.tokenContractERC20;

	// 		checkBatchValidation(_currentValset, _sigs, _tokenIds, _destinations, 
	// 		_fees, _batchNonce, _tokenContractERC721, _batchTimeout);
	// 		// Gravity g = Gravity(state_gravitySolAddress);

	// 		// Check that enough current validators have signed off on the transaction batch and valset

	// 		state_lastERC721BatchNonces[_tokenContractERC721] = _batchNonce;
	// 		executeERC721Batch(_tokenContractERC721, _tokenContractERC20, _tokenIds, _destinations, _fees); 
	// 	}

	// 	{
	// 		address _tokenContractERC721 = _tokenContracts.tokenContractERC721;
	// 		state_lastERC721EventNonce = state_lastERC721EventNonce + 1;
	// 		emit TxnERC721BatchExecutedEvent(_batchNonce, _tokenContractERC721, state_lastERC721EventNonce);
	// 	}
	// }


	function withdrawERC721(
		ValsetArgs calldata _currentValset,
		Signature[] calldata _sigs,
		uint256[] calldata _tokenIds,
		address[] calldata _transferTokenContracts,
		uint256[] calldata _feeAmounts, 
		address[] memory _feeTokenContracts,
		address[] memory _destinations
	) public returns (uint256) {
		Gravity g = Gravity(state_gravitySolAddress);

		LogicCallArgs memory logicArgs;
		logicArgs.transferAmounts = _tokenIds;
		logicArgs.transferTokenContracts = _transferTokenContracts;
		logicArgs.feeAmounts = _feeAmounts;
		logicArgs.feeTokenContracts = _feeTokenContracts;
		logicArgs.logicContractAddress = address(this);
		logicArgs.payload = abi.encodeWithSelector(bytes4(keccak256(
			"executeERC721Batch(address[],address,uint256[],address[],uint256[])")),
			_transferTokenContracts,address(this),_tokenIds,_destinations,_feeAmounts);
		logicArgs.timeOut = 4766922941000;
    	logicArgs.invalidationId = bytes32(uint256(uint160(_transferTokenContracts[0])) << 96);
    	logicArgs.invalidationNonce = 1;

		g.submitLogicCall(_currentValset, _sigs, logicArgs); 
		return 1;

	}
}
