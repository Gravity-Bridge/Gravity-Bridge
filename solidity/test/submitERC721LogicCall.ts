import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";
import { TestERC721A } from "../typechain/TestERC721A";
import { GravityERC721 } from "../typechain/GravityERC721";
import { deployContracts } from "../test-utils/deployERC721";
import {
  getSignerAddresses,
  signHash,
  examplePowers,
  ZeroAddress
} from "../test-utils/pure";
import { ERC721 } from "../typechain";

chai.use(solidity);
const { expect } = chai;


async function runTest(opts: {
  // Issues with the tx batch
  invalidationNonceNotHigher?: boolean;
  malformedTxBatch?: boolean;

  // Issues with the current valset and signatures
  nonMatchingCurrentValset?: boolean;
  badValidatorSig?: boolean;
  zeroedValidatorSig?: boolean;
  notEnoughPower?: boolean;
  barelyEnoughPower?: boolean;
  malformedCurrentValset?: boolean;
  timedOut?: boolean;
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
    testERC20,
    checkpoint: deployCheckpoint
  } = await deployContracts(gravityId, validators, powers);


  const TestGravityERC721Contract = await ethers.getContractFactory("GravityERC721");
  const ERC721LogicContract = (await TestGravityERC721Contract.deploy(gravity.address)) as GravityERC721;
  // We set its owner to the batch contract. 
  await ERC721LogicContract.transferOwnership(gravity.address);

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
    await gravityERC721.functions["sendERC721ToCosmos(address,string,uint256)"](
      testERC721.address,
      ethers.utils.formatBytes32String("myCosmosAddress"),
      1+i);
    tokenIds[i] = 1+i;
    destinations[i] = signers[i + 5].address;
    txAmounts[i] = 0;
  }

  let invalidationNonce = 1
  if (opts.invalidationNonceNotHigher) {
    invalidationNonce = 0
  }

  let timeOut = 4766922941000
  if (opts.timedOut) {
    timeOut = 0
  }


  // // // Call method
  // // // ===========
  // // // We have to give the logicBatch contract 5 coins for each tx, since it will transfer that
  // // // much to the logic contract.
  // // // We give msg.sender 1 coin in fees for each tx.
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

  let logicCallSubmitResult = await gravity.submitLogicCall(
    valset,
    sigs,
    logicCallArgs
  );

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
  it.only("throws on malformed current valset", async function () {
    await expect(runTest({ malformedCurrentValset: true })).to.be.revertedWith(
      "MalformedCurrentValidatorSet()"
    );
  });

  it.only("throws on invalidation nonce not incremented", async function () {
    await expect(runTest({ invalidationNonceNotHigher: true })).to.be.revertedWith(
      "InvalidLogicCallNonce(0, 0)"
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

  it.only("throws on timeout", async function () {
    await expect(runTest({ timedOut: true })).to.be.revertedWith(
      "LogicCallTimedOut()"
    );
  });

});

// This test produces a hash for the contract which should match what is being used in the Go unit tests. It's here for
// the use of anyone updating the Go tests.
describe("logicCall Go test hash", function () {
  it.only("produces good hash", async function () {


    // Prep and deploy contract
    // ========================
    const signers = await ethers.getSigners();
    const gravityId = ethers.utils.formatBytes32String("foo");
    const powers = [2934678416];
    const validators = signers.slice(0, powers.length);
    const {
      gravity,
      gravityERC721,
      testERC721,
      testERC20,
      checkpoint: deployCheckpoint
    } = await deployContracts(gravityId, validators, powers);


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
      await gravityERC721.functions["sendERC721ToCosmos(address,string,uint256)"](
        testERC721.address,
        ethers.utils.formatBytes32String("myCosmosAddress"),
        1+i);
      tokenIds[i] = 1+i;
      destinations[i] = signers[i + 5].address;
      txAmounts[i] = 0;
    }
  

    // Call method
    // ===========
    const methodName = ethers.utils.formatBytes32String(
      "logicCall"
    );
    let invalidationNonce = 1

    let timeOut = 4766922941000

    // let logicCallArgs = {
    //   transferAmounts: [1], // transferAmounts
    //   transferTokenContracts: [testERC20.address], // transferTokenContracts
    //   feeAmounts: [1], // feeAmounts
    //   feeTokenContracts: [testERC20.address], // feeTokenContracts
    //   logicContractAddress: "0x17c1736CcF692F653c433d7aa2aB45148C016F68", // logicContractAddress
    //   payload: ethers.utils.formatBytes32String("testingPayload"), // payloads
    //   timeOut,
    //   invalidationId: ethers.utils.formatBytes32String("invalidationId"), // invalidationId
    //   invalidationNonce: invalidationNonce // invalidationNonce
    // }

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


    const abiEncodedLogicCall = ethers.utils.defaultAbiCoder.encode(
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
    );
    const logicCallDigest = ethers.utils.keccak256(abiEncodedLogicCall);


    const sigs = await signHash(validators, logicCallDigest);
    const currentValsetNonce = 0;

    // TODO construct the easiest possible delegate contract that will
    // actually execute, existing ones are too large to bother with for basic
    // signature testing

    let valset = {
      validators: await getSignerAddresses(validators),
      powers,
      valsetNonce: currentValsetNonce,
      rewardAmount: 0,
      rewardToken: ZeroAddress
    }

    var res = await gravity.populateTransaction.submitLogicCall(
      valset,
      sigs,
      logicCallArgs
    )

    console.log("elements in logic call digest:", {
      "gravityId": gravityId,
      "logicMethodName": methodName,
      "transferAmounts": logicCallArgs.transferAmounts,
      "transferTokenContracts": logicCallArgs.transferTokenContracts,
      "feeAmounts": logicCallArgs.feeAmounts,
      "feeTokenContracts": logicCallArgs.feeTokenContracts,
      "logicContractAddress": logicCallArgs.logicContractAddress,
      "payload": logicCallArgs.payload,
      "timeout": logicCallArgs.timeOut,
      "invalidationId": logicCallArgs.invalidationId,
      "invalidationNonce": logicCallArgs.invalidationNonce
    })
    console.log("abiEncodedCall:", abiEncodedLogicCall)
    console.log("callDigest:", logicCallDigest)

    console.log("elements in logic call function call:", {
      "currentValidators": await getSignerAddresses(validators),
      "currentPowers": powers,
      "currentValsetNonce": currentValsetNonce,
      "sigs": sigs,
    })
    console.log("Function call bytes:", res.data)

  //check ownership of ERC721 tokens now transferred to signers
  for (let i = 0; i < numTxs; i++) {
    expect(
      (await testERC721.ownerOf(tokenIds[i]))
    ).to.equal(signers[i + 5].address);
  }

  })
});