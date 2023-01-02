// contracts/GLDToken.sol
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract DummyToken is ERC20, Ownable {
	constructor(uint256 initialSupply) ERC20("Dummy Token", "DY") {
		_mint(msg.sender, initialSupply);
	}

	function mint(address account, uint256 amount) external onlyOwner {
		_mint(account, amount);
	}
}
