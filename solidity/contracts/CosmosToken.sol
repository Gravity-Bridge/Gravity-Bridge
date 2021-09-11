//SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.7;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract CosmosERC20 is ERC20 {
	uint256 MAX_UINT = 2**256 - 1;
	uint8 private cosmosDecimals;

	function decimals() public view virtual override returns (uint8) {
		return cosmosDecimals;
	}

	constructor(
		address _gravityAddress,
		string memory _name,
		string memory _symbol,
		uint8 _decimals
	) ERC20(_name, _symbol) {
		cosmosDecimals = _decimals;
		_mint(_gravityAddress, MAX_UINT);
	}
}
