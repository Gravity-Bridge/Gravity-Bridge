import { Gravity } from "./typechain/Gravity";
import { ethers } from "ethers";
import fs from "fs";
import commandLineArgs from "command-line-args";
import axios from "axios";
import { exit } from "process";
// @ts-ignore
import TronWeb from 'tronweb';

const networkType = {
  ETHEREUM: "eth",
  TRON: "tron",
}

const args = commandLineArgs([
  // the ethernum node used to deploy the contract
  { name: "eth-node", type: String },
  // the cosmos node that will be used to grab the validator set via RPC (TODO),
  { name: "cosmos-node", type: String },
  // the Ethereum private key that will contain the gas required to pay for the contact deployment
  { name: "eth-privkey", type: String },
  // the gravity contract .json file
  { name: "contract", type: String },
  // the gravityERC721 contract .json file
  { name: "contractERC721", type: String },
  // test mode, if enabled this script deploys three ERC20 contracts for testing
  { name: "test-mode", type: String },
  { name: "evm-prefix", type: String, defaultValue: "" },
  { name: "admin", type: String, defaultValue: "0xD7F771664541b3f647CBA2be9Ab1Bc121bEEC913" },
  { name: "network-type", type: String, defaultValue: networkType.ETHEREUM },
  { name: 'headers', type: String }
]);

// 4. Now, the deployer script hits a full node api, gets the Eth signatures of the valset from the latest block, and deploys the Ethereum contract.
//     - We will consider the scenario that many deployers deploy many valid gravity eth contracts.
// 5. The deployer submits the address of the gravity contract that it deployed to Ethereum.
//     - The gravity module checks the Ethereum chain for each submitted address, and makes sure that the gravity contract at that address is using the correct source code, and has the correct validator set.
type Validator = {
  power: number;
  ethereum_address: string;
};
type ValsetTypeWrapper = {
  type: string;
  value: Valset;
}
type Valset = {
  members: Validator[];
  nonce: number;
};
type ABCIWrapper = {
  jsonrpc: string;
  id: string;
  result: ABCIResponse;
};
type ABCIResponse = {
  response: ABCIResult
}
type ABCIResult = {
  code: number
  log: string,
  info: string,
  index: string,
  value: string,
  height: string,
  codespace: string,
};
type StatusWrapper = {
  jsonrpc: string,
  id: string,
  result: NodeStatus
};
type NodeInfo = {
  protocol_version: JSON,
  id: string,
  listen_addr: string,
  network: string,
  version: string,
  channels: string,
  moniker: string,
  other: JSON,
};
type SyncInfo = {
  latest_block_hash: string,
  latest_app_hash: string,
  latest_block_height: Number
  latest_block_time: string,
  earliest_block_hash: string,
  earliest_app_hash: string,
  earliest_block_height: Number,
  earliest_block_time: string,
  catching_up: boolean,
}
type NodeStatus = {
  node_info: NodeInfo,
  sync_info: SyncInfo,
  validator_info: JSON,
};

// sets the gas price for all contract deployments
const overrides = {
  //gasPrice: 100000000000
}

async function deploy() {
  const provider = new ethers.providers.JsonRpcProvider(args["eth-node"]);
  let wallet = new ethers.Wallet(args["eth-privkey"], provider);

  const gravityIdString = await getGravityId();
  console.log("gravity id: ", gravityIdString)
  const gravityId = ethers.utils.formatBytes32String(gravityIdString);

  console.log("Starting Gravity contract deploy");
  const { abi, bytecode } = getContractArtifacts(args["contract"]);
  const factory = new ethers.ContractFactory(JSON.stringify(abi), JSON.stringify(bytecode), wallet);

  console.log("About to get latest Gravity valset");
  const latestValset = await getLatestValset();

  let eth_addresses = [];
  let powers = [];
  let powers_sum = 0;
  // this MUST be sorted uniformly across all components of Gravity in this
  // case we perform the sorting in module/x/gravity/keeper/types.go to the
  // output of the endpoint should always be sorted correctly. If you're
  // having strange problems with updating the validator set you should go
  // look there.
  for (let i = 0; i < latestValset.members.length; i++) {
    if (latestValset.members[i].ethereum_address == null) {
      continue;
    }
    eth_addresses.push(latestValset.members[i].ethereum_address);
    powers.push(latestValset.members[i].power);
    powers_sum += latestValset.members[i].power;
  }

  // 66% of uint32_max
  let vote_power = 2834678415;
  if (powers_sum < vote_power) {
    console.log("Refusing to deploy! Incorrect power! Please inspect the validator set below")
    console.log("If less than 66% of the current voting power has unset Ethereum Addresses we refuse to deploy")
    console.log(latestValset)
    exit(1)
  }

  const gravity = (await factory.deploy(
    // todo generate this randomly at deployment time that way we can avoid
    // anything but intentional conflicts
    gravityId,
    eth_addresses,
    powers,
    args["admin"],
    overrides
  )) as Gravity;

  await gravity.deployed();
  console.log("Gravity deployed at Address - ", gravity.address);
}

async function deployTron() {
  console.log(args['eth-node'], args['headers'], args['eth-privkey'])
  const tronWeb = new TronWeb({
    fullHost: args['eth-node'],
    headers: { "TRON-PRO-API-KEY": args['headers'] },
    privateKey: args['eth-privkey']
  })

  console.log("tron web: ", tronWeb.defaultAddress)

  const gravityIdString = await getGravityId();
  console.log("gravity id: ", gravityIdString)
  const gravityId = ethers.utils.formatBytes32String(gravityIdString);

  console.log("Starting Gravity contract deploy");
  const { abi, bytecode } = getContractArtifacts(args["contract"]);

  console.log("About to get latest Gravity valset");
  const latestValset = await getLatestValset();

  let eth_addresses = [];
  let powers = [];
  let powers_sum = 0;
  // this MUST be sorted uniformly across all components of Gravity in this
  // case we perform the sorting in module/x/gravity/keeper/types.go to the
  // output of the endpoint should always be sorted correctly. If you're
  // having strange problems with updating the validator set you should go
  // look there.
  for (let i = 0; i < latestValset.members.length; i++) {
    if (latestValset.members[i].ethereum_address == null) {
      continue;
    }
    eth_addresses.push(latestValset.members[i].ethereum_address);
    powers.push(latestValset.members[i].power);
    powers_sum += latestValset.members[i].power;
  }

  // 66% of uint32_max
  let vote_power = 2834678415;
  if (powers_sum < vote_power) {
    console.log("Refusing to deploy! Incorrect power! Please inspect the validator set below")
    console.log("If less than 66% of the current voting power has unset Ethereum Addresses we refuse to deploy")
    console.log(latestValset)
    exit(1)
  }

  // try {
  //   tronWeb.trx.sendTransaction("TPwTVfDDvmWSawsP7Ki1t3ecSBmaFeMMXc", 2000000000);
  // } catch (error) {
  //   console.log("error: ", error);
  // }

  try {
    const gravity = await tronWeb.contract().new({
      abi: abi,
      bytecode,
      feeLimit: 1300000000,
      callValue: 0,
      userFeePercentage: 50,
      parameters: [
        gravityId,
        eth_addresses,
        powers,
        args["admin"]
      ]
    });
    console.log("Gravity deployed at Address - ", gravity.address);
  } catch (error) {
    console.log("Error deploying a Gravity contract onto Tron network: ", error);
  }
}

function getContractArtifacts(path: string): { bytecode: string; abi: any } {
  var { bytecode, abi } = JSON.parse(fs.readFileSync(path, "utf8").toString());
  return { bytecode, abi };
}
const decode = (str: string): string => Buffer.from(str, 'base64').toString('binary');

async function getLatestValset(): Promise<Valset> {
  let block_height_request_string = args["cosmos-node"] + '/status';
  let block_height_response = await axios.get(block_height_request_string);
  let info: StatusWrapper = await block_height_response.data;
  let block_height = info.result.sync_info.latest_block_height;
  if (info.result.sync_info.catching_up) {
    console.log("This node is still syncing! You can not deploy using this validator set!");
    exit(1);
  }
  let request_string = args["cosmos-node"] + "/abci_query"
  let params = {
    params: {
      path: `\"/custom/gravity/currentValset/${args['evm-prefix']}\"`,
      height: block_height,
      prove: "false",
    }
  };
  let response = await axios.get(request_string, params);
  let valsets: ABCIWrapper = await response.data;


  // if in test mode retry the request as needed in some cases
  // the cosmos nodes do not start in time
  console.log('val set: ', valsets.result)
  console.log(decode(valsets.result.response.value));
  let valset: ValsetTypeWrapper = JSON.parse(decode(valsets.result.response.value))
  return valset.value;
}

async function getGravityId(): Promise<string> {
  let block_height_request_string = args["cosmos-node"] + "/status";
  let block_height_response = await axios.get(block_height_request_string);
  let info: StatusWrapper = await block_height_response.data;
  let block_height = info.result.sync_info.latest_block_height;
  if (info.result.sync_info.catching_up) {
    console.log(
      "This node is still syncing! You can not deploy using this gravityID!"
    );
    exit(1);
  }
  let request_string = args["cosmos-node"] + "/abci_query";
  let params = {
    params: {
      path: `\"/custom/gravity/gravityID/${args['evm-prefix']}\"`,
      height: block_height,
      prove: "false",
    },
  };

  let response = await axios.get(request_string, params);
  let gravityIDABCIResponse: ABCIWrapper = await response.data;

  // if in test mode retry the request as needed in some cases
  // the cosmos nodes do not start in time
  let gravityID: string = JSON.parse(
    decode(gravityIDABCIResponse.result.response.value)
  );
  return gravityID;
}

async function main() {
  const type = args['network-type'];
  switch (type) {
    case networkType.TRON:
      await deployTron();
      break;
    case networkType.ETHEREUM:
    default:
      await deploy();
      break;
  }
}

main();

// npx ts-node contract-deployer.ts --cosmos-node="http://localhost:26657" --eth-node=https://api.trongrid.io --eth-privkey= --contract=artifacts/contracts/Gravity.sol/Gravity.json --evm-prefix="tron-testnet" --headers abcd1234 --network-type tron --admin 0xD7F771664541b3f647CBA2be9Ab1Bc121bEEC913