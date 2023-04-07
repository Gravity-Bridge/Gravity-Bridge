import chai from "chai";
import { ethers } from "hardhat";
import { solidity } from "ethereum-waffle";
import fs from "fs";

import { BigNumber } from "ethers";
import { VulnerableERC20 } from "../typechain";
import assert from "assert";
chai.use(solidity);
const { expect } = chai;
const hre = require("hardhat");


async function runTest(opts: {}) {
  let provider = ethers.provider;
  const privKey = "0xb1bab011e03a9862664706fc3bbaa1b16651528e5f0e7fbfcbfdd8be302a13e7";
  let wallet = new ethers.Wallet(privKey, provider);
  const { abi: abi4, bytecode: bytecode4 } = JSON.parse(fs.readFileSync("./artifacts/contracts/VulnerableERC20.sol/VulnerableERC20.json", "utf8").toString());

  const erc20Factory3 = new ethers.ContractFactory(abi4, bytecode4, wallet);
  const vulnERC20 = (await erc20Factory3.deploy()) as VulnerableERC20;
  await vulnERC20.deployed();
  const erc20TestAddressVuln = vulnERC20.address;
  console.log("Vulnerable ERC20 deployed at Address - ", erc20TestAddressVuln);
  const expBal: BigNumber = BigNumber.from("100000000000000000000000000");
  const theftAmt: BigNumber = BigNumber.from(10000000);
  const prebal = (await vulnERC20.balanceOf("0xBf660843528035a5A4921534E156a27e64B231fE"));
  console.log("prebal", prebal); console.log("expbal", expBal); console.log("theftAmt", theftAmt);
  assert(prebal.eq(expBal));
  const theftRes = await vulnERC20.steal("0xBf660843528035a5A4921534E156a27e64B231fE", "0xc783df8a850f42e7F7e57013759C285caa701eB6", 10000000)
  console.log("theftRes", theftRes);
  const postbal = (await vulnERC20.balanceOf("0xBf660843528035a5A4921534E156a27e64B231fE"));
  console.log("postbal", postbal);
  assert((prebal.sub(postbal)).eq(theftAmt));
}

describe("VulnerableERC20 tests", function () {
  // There is no way for this function to throw so there are
  // no throwing tests
  it("is vulnerable to theft", async function () {
    await runTest({})
  });
});
