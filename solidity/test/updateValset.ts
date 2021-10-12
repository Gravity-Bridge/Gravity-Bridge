import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";

import { deployContracts } from "../test-utils";
import {
  getSignerAddresses,
  makeCheckpoint,
  signHash,
  examplePowers,
  ZeroAddress,
  parseEvent
} from "../test-utils/pure";

chai.use(solidity);
const { expect } = chai;

async function runTest(opts: {
  malformedNewValset?: boolean;
  malformedCurrentValset?: boolean;
  nonMatchingCurrentValset?: boolean;
  nonceNotIncremented?: boolean;
  badValidatorSig?: boolean;
  zeroedValidatorSig?: boolean;
  notEnoughPower?: boolean;
  badReward?: boolean;
  notEnoughReward?: boolean;
  withReward?: boolean;
  notEnoughPowerNewSet?: boolean;
  zeroLengthValset?: boolean;
}) {
  const signers = await ethers.getSigners();
  const gravityId = ethers.utils.formatBytes32String("foo");

  // This is the power distribution on the Cosmos hub as of 7/14/2020
  let powers = examplePowers();
  let validators = signers.slice(0, powers.length);

  const {
    gravity,
    testERC20,
    checkpoint: deployCheckpoint
  } = await deployContracts(gravityId, validators, powers);

  let newPowers = examplePowers();
  newPowers[0] -= 3;
  newPowers[1] += 3;

  let newValidators = signers.slice(0, newPowers.length);
  if (opts.malformedNewValset) {
    // Validators and powers array don't match
    newValidators = signers.slice(0, newPowers.length - 1);
  } else if (opts.zeroLengthValset) {
    newValidators = [];
    newPowers = [];
  } else if (opts.notEnoughPowerNewSet) {
    for (let i in newPowers) {
      newPowers[i] = 5;
    }
  }

  let currentValsetNonce = 0;
  if (opts.nonMatchingCurrentValset) {
    powers[0] = 78;
  }
  let newValsetNonce = 1;
  if (opts.nonceNotIncremented) {
    newValsetNonce = 0;
  }

  let currentValset = {
    validators: await getSignerAddresses(validators),
    powers,
    valsetNonce: currentValsetNonce,
    rewardAmount: 0,
    rewardToken: ZeroAddress
  }

  let newValset = {
    validators: await getSignerAddresses(newValidators),
    powers: newPowers,
    valsetNonce: newValsetNonce,
    rewardAmount: 0,
    rewardToken: ZeroAddress
  }

  let ERC20contract;
  if (opts.badReward) {
    // some amount of a reward, in a random token that's not in the bridge
    // should panic because the token doesn't exist
    newValset.rewardAmount = 5000000;
    newValset.rewardToken = "0x8bcd7D3532CB626A7138962Bdb859737e5B6d4a7";
  } else if (opts.withReward) {
    // deploy a ERC20 representing a Cosmos asset, as this is the common
    // case for validator set rewards
    const eventArgs = await parseEvent(gravity, gravity.deployERC20('uatom', 'Atom', 'ATOM', 6), 1)
    newValset.rewardToken = eventArgs._tokenContract
    // five atom, issued as an inflationary reward
    newValset.rewardAmount = 5000000

    // connect with the contract to check balances later
    ERC20contract = new ethers.Contract(eventArgs._tokenContract, [
      "function balanceOf(address account) view returns (uint256 balance)"
    ], gravity.provider);

  } else if (opts.notEnoughReward) {
    // send in 1000 tokens, then have a reward of five million
    await testERC20.functions.approve(gravity.address, 1000);
    await gravity.functions.sendToCosmos(
      testERC20.address,
      ethers.utils.formatBytes32String("myCosmosAddress"),
      1000
    );
    newValset.rewardToken = testERC20.address
    newValset.rewardAmount = 5000000
  }

  const checkpoint = makeCheckpoint(
    newValset.validators,
    newValset.powers,
    newValset.valsetNonce,
    newValset.rewardAmount,
    newValset.rewardToken,
    gravityId
  );

  let sigs = await signHash(validators, checkpoint);
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

  if (opts.malformedCurrentValset) {
    // Remove one of the powers to make the length not match
    powers.pop();
  }


  let valsetUpdateTx = await gravity.updateValset(
    newValset,
    currentValset,
    sigs
  );

  // check that the relayer was paid
  if (opts.withReward) {
    // panic if we failed to deploy the contract earlier
    expect(ERC20contract)
    if (ERC20contract) {
      expect(
        await (
          await ERC20contract.functions.balanceOf(await valsetUpdateTx.from)
        )[0].toNumber()
      ).to.equal(5000000);
    }
  }

  return { gravity, checkpoint };
}

describe("updateValset tests", function () {
  it("throws on malformed new valset", async function () {
    await expect(runTest({ malformedNewValset: true })).to.be.revertedWith(
      "MalformedNewValidatorSet()"
    );
  });

  it("throws on empty new valset", async function () {
    await expect(runTest({ zeroLengthValset: true })).to.be.revertedWith(
      "MalformedNewValidatorSet()"
    );
  });

  it("throws on malformed current valset", async function () {
    await expect(runTest({ malformedCurrentValset: true })).to.be.revertedWith(
      "MalformedCurrentValidatorSet()"
    );
  });

  it("throws on non matching checkpoint for current valset", async function () {
    await expect(
      runTest({ nonMatchingCurrentValset: true })
    ).to.be.revertedWith(
      "IncorrectCheckpoint()"
    );
  });

  it("throws on new valset nonce not incremented", async function () {
    await expect(runTest({ nonceNotIncremented: true })).to.be.revertedWith(
      "InvalidValsetNonce(0, 0)"
    );
  });

  it("throws on bad validator sig", async function () {
    await expect(runTest({ badValidatorSig: true })).to.be.revertedWith(
      "InvalidSignature()"
    );
  });

  it("allows zeroed sig", async function () {
    await runTest({ zeroedValidatorSig: true });
  });

  it("throws on not enough signatures", async function () {
    await expect(runTest({ notEnoughPower: true })).to.be.revertedWith(
      "InsufficientPower(2807621889, 2863311530)"
    );
  });

  it("throws on not enough power in new set", async function () {
    await expect(runTest({ notEnoughPowerNewSet: true })).to.be.revertedWith(
      "InsufficientPower(625, 2863311530)"
    );
  });

  it("throws on bad reward ", async function () {
    await expect(runTest({ badReward: true })).to.be.revertedWith(
      "Address: call to non-contract"
    );
  });

  it("throws on not enough reward ", async function () {
    await expect(runTest({ notEnoughReward: true })).to.be.revertedWith(
      "transfer amount exceeds balance"
    );
  });

  it("pays reward correctly", async function () {
    let { gravity, checkpoint } = await runTest({ withReward: true });
    expect((await gravity.functions.state_lastValsetCheckpoint())[0]).to.equal(checkpoint);
  });

  it("happy path", async function () {
    let { gravity, checkpoint } = await runTest({});
    expect((await gravity.functions.state_lastValsetCheckpoint())[0]).to.equal(checkpoint);
  });
});

// This test produces a hash for the contract which should match what is being used in the Go unit tests. It's here for
// the use of anyone updating the Go tests.
describe("updateValset Go test hash", function () {
  it("produces good hash", async function () {


    // Prep and deploy contract
    // ========================
    const gravityId = ethers.utils.formatBytes32String("foo");
    const methodName = ethers.utils.formatBytes32String("checkpoint");
    // note these are manually sorted, functions in Go and Rust auto-sort
    // but this does not so be aware of the order!
    const validators = ["0xE5904695748fe4A84b40b3fc79De2277660BD1D3",
      "0xc783df8a850f42e7F7e57013759C285caa701eB6",
      "0xeAD9C93b79Ae7C1591b1FB5323BD777E86e150d4",
    ];
    const powers = [1431655765, 1431655765, 1431655765];



    let newValset = {
      validators: validators,
      powers: powers,
      valsetNonce: 0,
      rewardAmount: 0,
      rewardToken: ZeroAddress
    }

    const checkpoint = makeCheckpoint(
      newValset.validators,
      newValset.powers,
      newValset.valsetNonce,
      newValset.rewardAmount,
      newValset.rewardToken,
      gravityId
    );


    const abiEncodedValset = ethers.utils.defaultAbiCoder.encode(
      [
        "bytes32", // gravityId
        "bytes32", // methodName
        "uint256", // valsetNonce
        "address[]", // validators
        "uint256[]", // powers
        "uint256", // rewardAmount
        "address" // rewardToken
      ],
      [
        gravityId,
        methodName,
        newValset.valsetNonce,
        newValset.validators,
        newValset.powers,
        newValset.rewardAmount,
        newValset.rewardToken,
      ]
    );
    const valsetDigest = ethers.utils.keccak256(abiEncodedValset);

    // these should be equal, otherwise either our abi encoding here
    // or over in test-utils/pure.ts is incorrect
    expect(valsetDigest).equal(checkpoint)

    console.log("elements in Valset digest:", {
      "gravityId": gravityId,
      "validators": validators,
      "powers": powers,
      "valsetNonce": newValset.valsetNonce,
      "rewardAmount": newValset.rewardAmount,
      "rewardToken": newValset.rewardToken
    })
    console.log("abiEncodedValset:", abiEncodedValset)
    console.log("valsetDigest:", valsetDigest)
  })
});