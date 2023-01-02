import { ethers } from "hardhat";

async function main() {

  const signers = await ethers.getSigners();

  // need to deploy token first locally
  const gravity = await ethers.getContractAt("Gravity", process.env.GRAVITY_CONTRACT || "0xEC0983BB79e3aca582E566e8F36c76713214F9a3", signers[0]);

  const result = await gravity.state_lastValsetCheckpoint();
  console.log(`latest valset checkpoint result: `, result);
}

// We recommend this pattern to be able to use async/await everywhere
// and properly handle errors.
main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});


