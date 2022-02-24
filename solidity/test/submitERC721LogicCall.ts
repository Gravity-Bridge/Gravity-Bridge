import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";
import { GravityERC721 } from "../typechain/GravityERC721";
import { deployContractsERC721 } from "../test-utils/deployERC721";
import {
  getSignerAddresses,
  signHash,
  examplePowers,
  ZeroAddress
} from "../test-utils/pure";

chai.use(solidity);
const { expect } = chai;


async function runTest(opts: {
  wrongCaller?: boolean;
  wrongERC721Owner?: boolean;
  ERC721NotExist?: boolean;
  ERC721NotInContract?: boolean;
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
    fakeGravity,
    testERC721,
    testERC20,
    checkpoint
  } = await deployContractsERC721(gravityId, validators, powers);

  const TestGravityERC721Contract = await ethers.getContractFactory("GravityERC721");
  const ERC721LogicContract = (await TestGravityERC721Contract.deploy(gravity.address)) as GravityERC721;

  // Transfer out to Cosmos, locking coins
  // =====================================
  await testERC20.functions.approve(gravity.address, 1000);
  await gravity.functions.sendToCosmos(
    testERC20.address,
    ethers.utils.formatBytes32String("myCosmosAddress"),
    1000
  );

  const numTxs = 10;
  const txPayloads = new Array(numTxs);
  const txAmounts = new Array(numTxs);
  const tokenIds = new Array(numTxs);
  const destinations = new Array(numTxs);

  for (let i = 0; i < numTxs; i++) {
    await testERC721.functions.approve(gravityERC721.address, 1+i);
    if (!opts.ERC721NotInContract) {
      await gravityERC721.functions["sendERC721ToCosmos(address,string,uint256)"](
        testERC721.address,
        ethers.utils.formatBytes32String("myCosmosAddress"),
        1+i);
    }
    tokenIds[i] = 1+i;
    destinations[i] = signers[i + 5].address;
    txAmounts[i] = 0;
  }

  // TestERC721A contract currently owns tokenIds 190-199
  // Therefore if we try to send these tokenIds from GravityERC721
  // Then we will get a wrong owner error
  if (opts.wrongERC721Owner) {
    const nftTokenOffset = 190; 
    for (let i = nftTokenOffset; i < nftTokenOffset+numTxs; i++) {
      tokenIds[i-nftTokenOffset] = i;
    }
   }
    // TokenIds 1000-1009 were never minted in the test contracts
    // Therefore we will get a does not exist error if we try to 
    // transfer them
    else if (opts.ERC721NotExist) {
    const nftTokenOffset = 1000; 
      for (let i = nftTokenOffset; i < nftTokenOffset+numTxs; i++) {
        tokenIds[i-nftTokenOffset] = i;
      }
    }

  let invalidationNonce = 1
  let timeOut = 4766922941000

  // // // Call method
  // // // ===========
  // // // We give msg.sender 10 coin in fees for each tx.
  const methodName = ethers.utils.formatBytes32String(
    "logicCall"
  );

  let logicCallArgs = {
    transferAmounts: [0], // transferAmounts
    transferTokenContracts: [testERC20.address], // transferTokenContracts
    feeAmounts: [numTxs], // feeAmounts
    feeTokenContracts: [testERC20.address], // feeTokenContracts
    logicContractAddress: gravityERC721.address, // logicContractAddress
    payload: gravityERC721.interface.encodeFunctionData("withdrawERC721", [testERC721.address, tokenIds, destinations]), // payloads
    timeOut,
    invalidationId: ethers.utils.hexZeroPad(testERC20.address, 32), // invalidationId
    invalidationNonce: invalidationNonce // invalidationNonce
  }


  const digest = ethers.utils.keccak256(ethers.utils.defaultAbiCoder.encode(
    [
      "bytes32", // gravityId
      "bytes32", // methodName
      "uint256[]", // transferAmounts
      "address[]", // transferTokenContracts
      "uint256[]", // feeAmounts
      "address[]", // feeTokenContracts
      "address", // logicContractAddress
      "bytes", // payload
      "uint256", // timeOut
      "bytes32", // invalidationId
      "uint256" // invalidationNonce
    ],
    [
      gravityId,
      methodName,
      logicCallArgs.transferAmounts,
      logicCallArgs.transferTokenContracts,
      logicCallArgs.feeAmounts,
      logicCallArgs.feeTokenContracts,
      logicCallArgs.logicContractAddress,
      logicCallArgs.payload,
      logicCallArgs.timeOut,
      logicCallArgs.invalidationId,
      logicCallArgs.invalidationNonce
    ]
  ));

  const sigs = await signHash(validators, digest);

  let currentValsetNonce = 0;

  let valset = {
    validators: await getSignerAddresses(validators),
    powers,
    valsetNonce: currentValsetNonce,
    rewardAmount: 0,
    rewardToken: ZeroAddress
  }

  let logicCallSubmitResult; 
  if (opts.wrongCaller) {
    logicCallSubmitResult = await fakeGravity.submitLogicCall(
      valset,
      sigs,
      logicCallArgs
    );
  }
  else {
    logicCallSubmitResult = await gravity.submitLogicCall(
      valset,
      sigs,
      logicCallArgs
    );
  }

  //check ownership of ERC721 tokens now transferred to signers
  for (let i = 0; i < numTxs; i++) {
    expect(
      (await testERC721.ownerOf(tokenIds[i]))
    ).to.equal(signers[i + 5].address);
  }

  // check that the relayer was paid
  expect(
    await (
      await testERC20.functions.balanceOf(await logicCallSubmitResult.from)
    )[0].toNumber()
  ).to.equal(9010);

  expect(
    (await testERC20.functions.balanceOf(await signers[20].getAddress()))[0].toNumber()
  ).to.equal(0);

  expect(
    (await testERC20.functions.balanceOf(gravity.address))[0].toNumber()
  ).to.equal(990);

  expect(
    (await testERC20.functions.balanceOf(ERC721LogicContract.address))[0].toNumber()
  ).to.equal(0);

  expect(
    (await testERC20.functions.balanceOf(await signers[0].getAddress()))[0].toNumber()
  ).to.equal(9010);
}

describe("submitLogicCall tests", function () {
  it("Happy Path", async function () {
    await runTest({})
  });

  it("throws on wrong caller", async function () {
    await expect(runTest({ wrongCaller: true })).to.be.revertedWith(
      "Can only call from Gravity.sol"
    );
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
