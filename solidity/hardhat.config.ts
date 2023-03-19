import "@nomiclabs/hardhat-waffle";
import "hardhat-gas-reporter";
import "hardhat-typechain";
import "hardhat-contract-sizer";
import { task, extendEnvironment } from "hardhat/config";
import {
  HardhatNetworkAccountsUserConfig,
  HardhatUserConfig,
} from "hardhat/types";
import { ethers } from "ethers";

task("accounts", "Prints the list of accounts", async (args, hre) => {
  const accounts = hre.getSigners();

  for (const account of accounts) {
    console.log(await account.getAddress());
    // console.log(await account.getAddress());
  }
});

let accounts: HardhatNetworkAccountsUserConfig | undefined = undefined;
if (process.env.MNEMONIC) {
  accounts = {
    mnemonic: process.env.MNEMONIC,
    path: "m/44'/60'/0'/0",
    initialIndex: 0,
    count: 20,
    passphrase: "",
  };
} else if (process.env.PRIVATE_KEY) {
  accounts = process.env.PRIVATE_KEY.split(/\s*,\s*/).map((pv) => ({
    privateKey: pv,
    balance: "10000000000000000000000",
  }));
}

// You have to export an object to set up your config
// This object can have the following optional entries:
// defaultNetwork, networks, solc, and paths.
// Go to https://buidler.dev/config/ to learn more
const config: HardhatUserConfig = {
  defaultNetwork: "hardhat",
  // This is a sample solc configuration that specifies which version of solc to use
  solidity: {
    compilers: [
      {
        version: "0.8.10",
        settings: {
          optimizer: {
            enabled: true,
          },
        },
      },
      {
        version: "0.8.12",
        settings: {
          optimizer: {
            enabled: true,
          },
        },
      },
    ],
  },
  networks: {
    hardhat: {
      chainId: 420,
      accounts,
      forking: {
        url: "https://rpc.ankr.com/eth_goerli",
        blockNumber: 8218229,
      },
      mining: {
        auto: false,
        interval: 2000,
      },
    },
  },
  typechain: {
    outDir: "typechain",
    target: "ethers-v5",
  },
  gasReporter: {
    enabled: true,
  },
  mocha: {
    timeout: 2000000,
  },
};

declare module "hardhat/types/runtime" {
  export interface HardhatRuntimeEnvironment {
    provider: ethers.providers.Web3Provider;
    getSigner: (
      addressOrIndex?: string | number
    ) => ethers.providers.JsonRpcSigner;
    getSigners: (num?: number) => ethers.providers.JsonRpcSigner[];
  }
}

extendEnvironment((hre) => {
  // @ts-ignore
  hre.provider = new ethers.providers.Web3Provider(hre.network.provider);
  hre.getSigners = (num = 20) =>
    [...new Array(num)].map((_, i) => hre.provider.getSigner(i));
  hre.getSigner = (addressOrIndex) => hre.provider.getSigner(addressOrIndex);
});

export default config;
