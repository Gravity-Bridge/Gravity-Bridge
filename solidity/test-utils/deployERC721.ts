import { Gravity } from "../typechain/Gravity";
import { TestERC20A } from "../typechain/TestERC20A";
import { GravityERC721} from "../typechain/GravityERC721";
import { TestERC721A } from "../typechain/TestERC721A";
import { ethers } from "hardhat";
import { makeCheckpoint, getSignerAddresses, ZeroAddress } from "./pure";
import { Signer } from "ethers";
import { TestFakeGravity } from "../typechain/TestFakeGravity";
import { deployContracts } from "./index";


export async function deployContractsERC721(
  gravityId: string = "foo",
  validators: Signer[],
  powers: number[],
) {

  const {
    gravity,
    testERC20,
    checkpoint
  } = await deployContracts(gravityId, validators, powers);

  const TestERC721 = await ethers.getContractFactory("TestERC721A");
  const testERC721= (await TestERC721.deploy()) as TestERC721A;

  // const Gravity = await ethers.getContractFactory("Gravity");

  // const valAddresses = await getSignerAddresses(validators);

  // const checkpoint = makeCheckpoint(valAddresses, powers, 0, 0, ZeroAddress, gravityId);

  // const gravity = (await Gravity.deploy(
  //   gravityId,
  //   await getSignerAddresses(validators),
  //   powers,
  // )) as Gravity;

  // await gravity.deployed();

  const GravityERC721 = await ethers.getContractFactory("GravityERC721");
  const gravityERC721 = (await GravityERC721.deploy(
    gravity.address
  )) as GravityERC721;

  const FakeGravity = await ethers.getContractFactory("TestFakeGravity");
  const fakeGravity= (await FakeGravity.deploy(
    gravityId,
    await getSignerAddresses(validators),
    powers
  )) as TestFakeGravity;

  return { gravity, gravityERC721, fakeGravity, testERC721,  testERC20, checkpoint };
}
