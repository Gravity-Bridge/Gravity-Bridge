// import { ethers } from "hardhat";

// async function main() {

//   const signers = await ethers.getSigners();

//   // need to deploy token first locally
//   const dummy = await ethers.getContractAt("DummyToken", process.env.DUMMY_TOKEN_CONTRACT || "0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef", signers[0]);

//   const result = await dummy.balanceOf("0xc9B6f87d637d4774EEB54f8aC2b89dBC3D38226b");
//   console.log(`ERC20 balance of Dummy token: `, result.toString());
// }

// // We recommend this pattern to be able to use async/await everywhere
// // and properly handle errors.
// main().catch((error) => {
//   console.error(error);
//   process.exitCode = 1;
// });

import Web3 from 'web3';
import dummyArtifacts from '../artifacts/contracts/DummyToken.sol/DummyToken.json';
const web3 = new Web3(process.env.JSON_RPC || 'http://localhost:8545');

function getContract() {
  return new web3.eth.Contract(dummyArtifacts.abi as any, process.env.DUMMY_TOKEN_CONTRACT || "0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef");
}

async function queryDummySymbol() {
  const dummyToken = getContract();
  const result = await dummyToken.methods.balanceOf("0xc9B6f87d637d4774EEB54f8aC2b89dBC3D38226b").call();
  console.log("result: ", result)
}

queryDummySymbol();