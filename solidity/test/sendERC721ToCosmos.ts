import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";

import { deployContractsERC721 } from "../test-utils/deployERC721";
import {
  examplePowers
} from "../test-utils/pure";
import { Gravity, GravityERC721, TestERC721A } from "../typechain";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";


chai.use(solidity);
const { expect } = chai;

async function runTest(opts: {
  wrongERC721Owner?: boolean;
  ERC721NotInContract?: boolean;
  ERC721NotExist?: boolean; 
}) {

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
  if (!opts.wrongERC721Owner){
    await testERC721.functions.approve(gravityERC721.address, 190);
  }
    await firstCall(gravityERC721, testERC721, signers, gravity);
    let secondERC721 = await getSecondERC721(testERC721, gravityERC721, opts.ERC721NotExist, opts.ERC721NotInContract);
    if (!opts.ERC721NotExist && !opts.ERC721NotInContract) {
      await testERC721.functions.approve(gravityERC721.address, secondERC721);
    }
    await secondCall(gravityERC721, testERC721, signers, gravity, secondERC721);
}

// this will get the tokenId of the second ERC721 token to send to Cosmos
// branches on the different test conditions
async function getSecondERC721(testERC721: TestERC721A, gravityERC721: GravityERC721,
   ERC721NotExist?: boolean, ERC721NotInContract?: boolean) {
let secondERC721 = 191;
  if (ERC721NotExist) {
      secondERC721 = 1000; 
  }
  else if(ERC721NotInContract) {
    secondERC721 = 190; 
  }
  return secondERC721
}

async function firstCall(gravityERC721: GravityERC721, testERC721: TestERC721A,
   signers: SignerWithAddress[], gravity: Gravity) {
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
}

async function secondCall(gravityERC721: GravityERC721, testERC721: TestERC721A,
  signers: SignerWithAddress[], gravity: Gravity, secondERC721: number){
  await expect(gravityERC721.functions["sendERC721ToCosmos(address,string,uint256)"](
    testERC721.address,
    ethers.utils.formatBytes32String("myCosmosAddress"),
    secondERC721
  )).to.emit(gravityERC721, 'SendERC721ToCosmosEvent').withArgs(
      testERC721.address,
      await signers[0].getAddress(),
      ethers.utils.formatBytes32String("myCosmosAddress"),
      secondERC721, 
      3
    );
    expect((await testERC721.functions["ownerOf(uint256)"](secondERC721))[0]).to.equal(gravityERC721.address);
    expect((await gravity.functions.state_lastEventNonce())[0]).to.equal(1);
    expect((await gravityERC721.functions.state_lastERC721EventNonce())[0]).to.equal(3);
}

describe("sendERC721ToCosmos tests", function () {
  it("happy path", async function () {
    await runTest({})
  });

  it("throws on Wrong NFT owner", async function () {
    await expect(runTest({ wrongERC721Owner: true })).to.be.revertedWith(
      "ERC721: transfer caller is not owner nor approved"
    );
  });

  it("throws on NFT not in contract", async function () {
    await expect(runTest({ ERC721NotInContract: true })).to.be.revertedWith(
      "ERC721: transfer of token that is not own"
    );
  });

  it("throws on nonexistent token", async function () {
    await expect(runTest({ ERC721NotExist: true })).to.be.revertedWith(
      "ERC721: operator query for nonexistent token"
    );
  });
});
