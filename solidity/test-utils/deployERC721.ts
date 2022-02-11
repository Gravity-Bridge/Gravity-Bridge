import { Gravity } from "../typechain/Gravity";
import { GravityERC721} from "../typechain/GravityERC721";
import { TestERC721A } from "../typechain/TestERC721A";
import { ethers } from "hardhat";
import { makeCheckpoint, getSignerAddresses, ZeroAddress } from "./pure";
import { Signer } from "ethers";

type DeployContractsOptions = {
  corruptSig?: boolean;
};

export async function deployContracts(
  gravityId: string = "foo",
  validators: Signer[],
  powers: number[],
  opts?: DeployContractsOptions
) {
  // enable automining for these tests
  await ethers.provider.send("evm_setAutomine", [true]);

//   const TestERC20 = await ethers.getContractFactory("TestERC20A");
//   const testERC20 = (await TestERC20.deploy()) as TestERC20A;

  const TestERC721 = await ethers.getContractFactory("TestERC721A");
  const testERC721= (await TestERC721.deploy()) as TestERC721A;

  const Gravity = await ethers.getContractFactory("Gravity");

  const valAddresses = await getSignerAddresses(validators);

  const checkpoint = makeCheckpoint(valAddresses, powers, 0, 0, ZeroAddress, gravityId);

  const gravity = (await Gravity.deploy(
    gravityId,
    await getSignerAddresses(validators),
    powers,
  )) as Gravity;

  await gravity.deployed();

  const GravityERC721 = await ethers.getContractFactory("GravityERC721");
  const gravityERC721 = (await GravityERC721.deploy(
    gravity.address
  )) as GravityERC721;

  return { gravity, gravityERC721, testERC721, checkpoint };
}
