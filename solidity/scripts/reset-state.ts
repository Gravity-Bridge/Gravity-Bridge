import { ethers } from "hardhat";

async function main() {

  const signers = await ethers.getSigners();

  // need to deploy token first locally
  const gravity = await ethers.getContractAt("Gravity", "0xa49e040d7b8F045B090306C88aEF48955404B2e8", signers[0]);

  const result = await gravity.resetState("0xf48007ea0F3AA4d2A59DFb4473dd30f90488c8Ef", {});
  console.log(`stake result: `, result);
}

// We recommend this pattern to be able to use async/await everywhere
// and properly handle errors.
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});


