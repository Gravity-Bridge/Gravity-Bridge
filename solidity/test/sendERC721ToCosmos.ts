import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";

import { deployContracts } from "../test-utils/deployERC721";
import {
  getSignerAddresses,
  makeCheckpoint,
  signHash,
  makeTxBatchHash,
  examplePowers
} from "../test-utils/pure";

chai.use(solidity);
const { expect } = chai;


async function runTest(opts: {}) {


  // Prep and deploy contract
  // ========================
  const signers = await ethers.getSigners();
  const gravityId = ethers.utils.formatBytes32String("foo");
  // This is the power distribution on the Cosmos hub as of 7/14/2020
  let powers = examplePowers();
  let validators = signers.slice(0, powers.length);
  const {
    gravity,
    gravityERC721,
    testERC721,
    checkpoint: deployCheckpoint
  } = await deployContracts(gravityId, validators, powers);


  // Transfer out to Cosmos, locking coins
  // =====================================
  await testERC721.functions.approve(gravity.address, 190);
//   await expect(gravity.functions.sendToCosmos(
//     testERC20.address,
//     ethers.utils.formatBytes32String("myCosmosAddress"),
//     1000
//   )).to.emit(gravity, 'SendToCosmosEvent').withArgs(
//       testERC20.address,
//       await signers[0].getAddress(),
//       ethers.utils.formatBytes32String("myCosmosAddress"),
//       1000, 
//       2
//     );

}

describe("sendERC721ToCosmos tests", function () {
  it("works right", async function () {
    await runTest({})
  });
});