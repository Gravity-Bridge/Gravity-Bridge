//SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.10;

import "hardhat/console.sol";
import "./Gravity.sol";

contract TestFakeGravity is Gravity {

    constructor(
        bytes32 _gravityId,
		// The validator set, not in valset args format since many of it's
		// arguments would never be used in this case
		address[] memory _validators,
		uint256[] memory _powers
    ) Gravity(
        _gravityId,
		_validators,
		_powers
        ) {
    }
}
