import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";

import { deployContractsERC721 } from "../test-utils/deployERC721";
import {
  examplePowers
} from "../test-utils/pure";
import { GravityERC721 } from "../typechain";

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
    fakeGravity,
    checkpoint
  } = await deployContractsERC721(gravityId, validators, powers);


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
    expect((await testERC721.functions["ownerOf(uint256)"](190))[0]).to.equal(gravityERC721.address);
    expect((await gravity.functions.state_lastEventNonce())[0]).to.equal(1);
    expect((await gravityERC721.functions.state_lastERC721EventNonce())[0]).to.equal(2);

    await testERC721.functions.approve(gravityERC721.address, 191);
    await expect(gravityERC721.functions["sendERC721ToCosmos(address,string,uint256)"](
      testERC721.address,
      ethers.utils.formatBytes32String("myCosmosAddress"),
      191
    )).to.emit(gravityERC721, 'SendERC721ToCosmosEvent').withArgs(
        testERC721.address,
        await signers[0].getAddress(),
        ethers.utils.formatBytes32String("myCosmosAddress"),
        191, 
        3
      );
      expect((await testERC721.functions["ownerOf(uint256)"](191))[0]).to.equal(gravityERC721.address);
      expect((await gravity.functions.state_lastEventNonce())[0]).to.equal(1);
      expect((await gravityERC721.functions.state_lastERC721EventNonce())[0]).to.equal(3);
}

describe("sendERC721ToCosmos tests", function () {
  it.only("works right", async function () {
    await runTest({})
  });
});
