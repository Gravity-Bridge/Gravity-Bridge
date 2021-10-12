import { ethers } from "hardhat";
import { BigNumberish } from "ethers";
import { Signer } from "ethers";
import { ContractTransaction, utils } from 'ethers';

export const ZeroAddress: string = "0x0000000000000000000000000000000000000000"

export async function getSignerAddresses(signers: Signer[]) {
  return await Promise.all(signers.map(signer => signer.getAddress()));
}


export function makeCheckpoint(
  validators: string[],
  powers: BigNumberish[],
  valsetNonce: BigNumberish,
  rewardAmount: BigNumberish,
  rewardToken: string,
  gravityId: string
) {
  const methodName = ethers.utils.formatBytes32String("checkpoint");

  let abiEncoded = ethers.utils.defaultAbiCoder.encode(
    ["bytes32", "bytes32", "uint256", "address[]", "uint256[]", "uint256", "address"],
    [gravityId, methodName, valsetNonce, validators, powers, rewardAmount, rewardToken]
  );

  let checkpoint = ethers.utils.keccak256(abiEncoded);

  return checkpoint;
}

export type Sig = {
  v: number,
  r: string,
  s: string
};

export async function signHash(signers: Signer[], hash: string) {
  let sigs: Sig[] = [];

  for (let i = 0; i < signers.length; i = i + 1) {
    const sig = await signers[i].signMessage(ethers.utils.arrayify(hash));
    const address = await signers[i].getAddress();

    const splitSig = ethers.utils.splitSignature(sig);
    sigs.push({ v: splitSig.v!, r: splitSig.r, s: splitSig.s });
  }

  return sigs;
}

export function makeTxBatchHash(
  amounts: number[],
  destinations: string[],
  fees: number[],
  nonces: number[],
  gravityId: string
) {
  const methodName = ethers.utils.formatBytes32String("transactionBatch");

  let abiEncoded = ethers.utils.defaultAbiCoder.encode(
    ["bytes32", "bytes32", "uint256[]", "address[]", "uint256[]", "uint256[]"],
    [gravityId, methodName, amounts, destinations, fees, nonces]
  );

  // console.log(abiEncoded);

  let txHash = ethers.utils.keccak256(abiEncoded);

  return txHash;
}

export async function parseEvent(contract: any, txPromise: Promise<ContractTransaction>, eventOrder: number) {
  const tx = await txPromise
  const receipt = await contract.provider.getTransactionReceipt(tx.hash!)
  let args = (contract.interface as utils.Interface).parseLog(receipt.logs![eventOrder]).args

  // Get rid of weird quasi-array keys
  const acc: any = {}
  args = Object.keys(args).reduce((acc, key) => {
    if (Number.isNaN(parseInt(key, 10)) && key !== 'length') {
      acc[key] = args[key]
    }
    return acc
  }, acc)

  return args
}

export function examplePowers(): number[] {
  return [
    303654379,
    266717637,
    261134176,
    188549183,
    176952764,
    174805279,
    137009543,
    134003064,
    133573567,
    130137591,
    105656262,
    103508777,
    96207328,
    91482861,
    83322418,
    75161975,
    74302981,
    73014490,
    66142538,
    63995053,
    59700083,
    52828131,
    51110143,
    48533161,
    47244670,
    45956179,
    45097185,
    44667688,
    39513724,
    38654730,
    37795736,
    37795736,
    37795736,
    36507245,
    36507245,
    36077748,
    35218754,
    30064790,
    28776299,
    27487808,
    25340323,
    24910826,
    24051832,
    23622335,
    22333844,
    22333844,
    22333844,
    21474850,
    21045353,
    18897868,
    18038874,
    17179880,
    16750383,
    16320886,
    15891389,
    15891389,
    15461892,
    15032395,
    14602898,
    14173401,
    14173401,
    14173401,
    13743904,
    13314407,
    12884910,
    12884910,
    12455413,
    12025916,
    11596419,
    11166922,
    10737425,
    10307928,
    9878431,
    9878431,
    9448934,
    9448934,
    9448934,
    9019437,
    9019437,
    8589940,
    8160443,
    7730946,
    7301449,
    6871952,
    6012958,
    6012958,
    5583461,
    5583461,
    4724467,
    4294970,
    4294970,
    4294970,
    4294970,
    4294970,
    3865473,
    3435976,
    3435976,
    3006479,
    3006479,
    3006479,
    2576982,
    2576982,
    2147485,
    2147485,
    2147485,
    2147485,
    2147485,
    2147485,
    1717988,
    1717988,
    1288491,
    858994,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
    429497,
  ];
}
