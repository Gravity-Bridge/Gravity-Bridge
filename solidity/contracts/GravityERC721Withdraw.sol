pragma solidity 0.8.10;

import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import { ERC721Holder } from "@openzeppelin/contracts/token/ERC721/utils/ERC721Holder.sol";

interface GravityERC721WithdrawInterface {
	function executeERC721Batch(
		address[] calldata _transferTokenContracts,
        address _ERC721HolderAddress,
		uint256[] calldata _tokenIds,
		address[] calldata _destinations,
		uint256[] calldata _fees) external;
} 

contract GravityERC721Withdraw is GravityERC721WithdrawInterface {

	function executeERC721Batch (
		address[] calldata _transferTokenContracts,
        address _ERC721HolderAddress,
		uint256[] calldata _tokenIds,
		address[] calldata _destinations,
		uint256[] calldata _fees
	) external {
		
		// Send transaction amounts to destinations
		uint256 totalFee;
		for (uint256 i = 0; i < _tokenIds.length; i++) {
			ERC721(_transferTokenContracts[i]).safeTransferFrom(_ERC721HolderAddress, _destinations[i], _tokenIds[i]);
			totalFee = totalFee + _fees[i];
		}
	}

}