import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";
import { deployContracts } from "../test-utils";
import {
  examplePowers
} from "../test-utils/pure";
import { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers";

chai.use(solidity);
const { expect } = chai;

describe("constructor tests", function () {
  it("throws on malformed valset", async function () {
    const signers = await ethers.getSigners();
    const gravityId = ethers.utils.formatBytes32String("foo");

    // This is the power distribution on the Cosmos hub as of 7/14/2020
    let powers = examplePowers();
    let validators = signers.slice(0, powers.length - 1);

    await expect(
      deployContracts(gravityId, validators, powers,)
    ).to.be.revertedWith("MalformedCurrentValidatorSet()");
  });

  it("throws on insufficient power", async function () {
    const signers = await ethers.getSigners();
    const gravityId = ethers.utils.formatBytes32String("foo");

    // This is the power distribution on the Cosmos hub as of 7/14/2020
    let powers = examplePowers().slice(0, 2);
    let validators = signers.slice(0, 2);


    await expect(
      deployContracts(gravityId, validators, powers)
    ).to.be.revertedWith(
      "InsufficientPower(570372016, 2863311530)"
    );
  });

  it("throws on empty validator set", async function () {
    const signers = await ethers.getSigners();
    const gravityId = ethers.utils.formatBytes32String("foo");

    // This is the power distribution on the Cosmos hub as of 7/14/2020
    let powers: number[] = [];
    let validators: SignerWithAddress[] = [];


    await expect(
      deployContracts(gravityId, validators, powers)
    ).to.be.revertedWith(
      "MalformedCurrentValidatorSet()"
    );
  });
});
