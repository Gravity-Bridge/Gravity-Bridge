#!/bin/bash
npx ts-node \
contract-deployer.ts \
--cosmos-node="http://localhost:1317" \
--eth-node="http://localhost:8545" \
--eth-privkey="0xb1bab011e03a9862664706fc3bbaa1b16651528e5f0e7fbfcbfdd8be302a13e7" \
--contract=artifacts/contracts/Gravity.sol/Gravity.json \
--contractERC721=artifacts/contracts/GravityERC721.sol/GravityERC721.json \
--contractERC20A=artifacts/contracts/TestERC20A.sol/TestERC20A.json \
--contractERC20B=artifacts/contracts/TestERC20B.sol/TestERC20B.json \
--contractERC20C=artifacts/contracts/TestERC20C.sol/TestERC20C.json \
--test-mode=true
