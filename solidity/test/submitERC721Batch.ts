import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";

import { deployContracts } from "../test-utils/deployERC721";
import { GravityERC721 } from "../typechain";

import {
  getSignerAddresses,
  makeCheckpoint,
  signHash,
  makeTxBatchHash,
  examplePowers,
  ZeroAddress,
} from "../test-utils/pure";

chai.use(solidity);
const { expect } = chai;

async function runTest(opts: {
  // Issues with the tx batch
  batchNonceNotHigher?: boolean;
  malformedTxBatch?: boolean;

  // Issues with the current valset and signatures
  nonMatchingCurrentValset?: boolean;
  badValidatorSig?: boolean;
  zeroedValidatorSig?: boolean;
  notEnoughPower?: boolean;
  barelyEnoughPower?: boolean;
  malformedCurrentValset?: boolean;
  batchTimeout?: boolean;
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
    checkpoint: deployCheckpoint
  } = await deployContracts(gravityId, validators, powers);


  // Prepare batch
  // ===============================
  const numTxs = 10;
  const txDestinationsInt = new Array(numTxs);
  const txFees = new Array(numTxs);
  
  let erc721counter = 1;
  const txIds = new Array(numTxs);
  for (let i = 0; i < numTxs; i++) {
    await testERC721.functions.approve(gravityERC721.address, erc721counter+i);
    await gravityERC721.functions["sendERC721ToCosmos(address,string,uint256)"](
      testERC721.address,
      ethers.utils.formatBytes32String("myCosmosAddress"),
      erc721counter+i
    )
    txFees[i] = 1;
    txIds[i] = erc721counter+i;
    txDestinationsInt[i] = signers[i + 5];
  }
  const txDestinations = await getSignerAddresses(txDestinationsInt);
  if (opts.malformedTxBatch) {
    // Make the fees array the wrong size
    txFees.pop();
  }

  let batchTimeout = ethers.provider.blockNumber + 1000;
  if (opts.batchTimeout) {
    batchTimeout = ethers.provider.blockNumber - 1;
  }
  let batchNonce = 1;
  if (opts.batchNonceNotHigher) {
    batchNonce = 0;
  }

  // Call method
  // ===========
  const methodName = ethers.utils.formatBytes32String("transactionBatch");
  let abiEncoded = ethers.utils.defaultAbiCoder.encode(
    [
      "bytes32",
      "bytes32",
      "uint256[]",
      "address[]",
      "uint256[]",
      "uint256",
      "address",
      "uint256",
    ],
    [
      gravityId,
      methodName,
      txIds,
      txDestinations,
      txFees,
      batchNonce,
      testERC721.address,
      batchTimeout,
    ]
  );
  let digest = ethers.utils.keccak256(abiEncoded);
  let sigs = await signHash(validators, digest);
  let currentValsetNonce = 0;
  if (opts.nonMatchingCurrentValset) {
    // Wrong nonce
    currentValsetNonce = 420;
  }
  if (opts.malformedCurrentValset) {
    // Remove one of the powers to make the length not match
    powers.pop();
  }
  if (opts.badValidatorSig) {
    // Switch the first sig for the second sig to screw things up
    sigs[1].v = sigs[0].v;
    sigs[1].r = sigs[0].r;
    sigs[1].s = sigs[0].s;
  }
  if (opts.zeroedValidatorSig) {
    // Switch the first sig for the second sig to screw things up
    sigs[1].v = sigs[0].v;
    sigs[1].r = sigs[0].r;
    sigs[1].s = sigs[0].s;
    // Then zero it out to skip evaluation
    sigs[1].v = 0;
  }
  if (opts.notEnoughPower) {
    // zero out enough signatures that we dip below the threshold
    sigs[1].v = 0;
    sigs[2].v = 0;
    sigs[3].v = 0;
    sigs[5].v = 0;
    sigs[6].v = 0;
    sigs[7].v = 0;
    sigs[9].v = 0;
    sigs[11].v = 0;
    sigs[13].v = 0;
  }
  if (opts.barelyEnoughPower) {
    // Stay just above the threshold
    sigs[1].v = 0;
    sigs[2].v = 0;
    sigs[3].v = 0;
    sigs[5].v = 0;
    sigs[6].v = 0;
    sigs[7].v = 0;
    sigs[9].v = 0;
    sigs[11].v = 0;
  }

  let valset = {
    validators: await getSignerAddresses(validators),
    powers,
    valsetNonce: currentValsetNonce,
    rewardAmount: 0,
    rewardToken: ZeroAddress
  }

  let batchSubmitTx = await gravityERC721.submitERC721Batch(
    valset,
    sigs,
    txIds,
    txDestinations,
    txFees,
    batchNonce,
    testERC721.address,
    batchTimeout
  );
}

describe("submitBatch tests", function () {
  it.only("throws on batch nonce not incremented", async function () {
      await expect(runTest({ batchNonceNotHigher: true })).to.be.revertedWith(
        "InvalidBatchNonce(0, 0)"
      );
    });
    
  it.only("throws on malformed current valset", async function () {
    await expect(runTest({ malformedCurrentValset: true })).to.be.revertedWith(
      "MalformedCurrentValidatorSet()"
    );
  });
  
  it.only("throws on malformed txbatch", async function () {
  await expect(runTest({ malformedTxBatch: true })).to.be.revertedWith(
    "MalformedBatch()"
    );
  });
    
  it.only("throws on timeout batch", async function () {
  await expect(runTest({ batchTimeout: true })).to.be.revertedWith(
    "BatchTimedOut()"
    );
  });

  it.only("throws on non matching checkpoint for current valset", async function () {
  await expect(
    runTest({ nonMatchingCurrentValset: true })
    ).to.be.revertedWith(
    "IncorrectCheckpoint()"
    );
  });

  it.only("throws on bad validator sig", async function () {
    await expect(runTest({ badValidatorSig: true })).to.be.revertedWith(
      "InvalidSignature()"
    );
  });

  it.only("allows zeroed sig", async function () {
    await runTest({ zeroedValidatorSig: true });
  });

  it.only("throws on not enough signatures", async function () {
    await expect(runTest({ notEnoughPower: true })).to.be.revertedWith(
      "InsufficientPower(2807621889, 2863311530)"
    );
  });

  it.only("does not throw on barely enough signatures", async function () {
    await runTest({ barelyEnoughPower: true });
  });
})

