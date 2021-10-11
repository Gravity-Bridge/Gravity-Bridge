//SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.9;

import "hardhat/console.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

contract TestLogicContract is Ownable {
	address state_tokenContract;

	constructor(address _tokenContract) {
		state_tokenContract = _tokenContract;
	}

	function transferTokens(
		address _to,
		uint256 _a,
		uint256 _b
	) public onlyOwner {
		IERC20(state_tokenContract).transfer(_to, _a + _b);
		console.log("Sent Tokens");
	}
}
