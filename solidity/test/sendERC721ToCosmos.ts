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

// event SendERC721ToCosmosEvent(
//     address indexed _tokenContract,
//     address indexed _sender,
//     string _destination,
//     uint256 _tokenId,
//     uint256 _eventNonce
// );

async function runTest(opts: {}) {

    console.log("in test");

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
  await testERC721.functions.approve(gravityERC721.address, 190);
  await expect(gravityERC721.functions["sendERC721ToCosmos(address,string,uint256)"](
    testERC721.address,
    ethers.utils.formatBytes32String("myCosmosAddress"),
    190
  )).to.emit(gravityERC721, 'SendERC721ToCosmosEvent').withArgs(
      testERC721.address,
      await signers[0].getAddress(),
      ethers.utils.formatBytes32String("myCosmosAddress"),
      190, 
      2
    );

}

describe("sendERC721ToCosmos tests", function () {
  it.only("works right", async function () {
    await runTest({})
  });
});