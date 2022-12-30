import "@nomiclabs/hardhat-waffle";
import "hardhat-gas-reporter";
import "hardhat-typechain";
import { task } from "hardhat/config";

task("accounts", "Prints the list of accounts", async (args, hre) => {
  const accounts = await hre.ethers.getSigners();

  for (const account of accounts) {
    console.log(account.address);
  }
});

// // This is a sample Buidler task. To learn how to create your own go to
// // https://buidler.dev/guides/create-task.html
// task("accounts", "Prints the list of accounts", async (taskArgs, bre) => {
//   const accounts = await bre.ethers.getSigners();

//   for (const account of accounts) {
//     console.log(await account.getAddress());
//   }
// });

// You have to export an object to set up your config
// This object can have the following optional entries:
// defaultNetwork, networks, solc, and paths.
// Go to https://buidler.dev/config/ to learn more
module.exports = {
  // This is a sample solc configuration that specifies which version of solc to use
  solidity: {
    version: "0.8.10",
    settings: {
      optimizer: {
        enabled: true
      }
    }
  },
  networks: {
    goerli: {
      url: "https://rpc.ankr.com/eth_goerli",
      accounts: ["0xbbfb76c92cd13796899f63dc6ead6d2420e8d0bc502d42bd5773c2d4b8897f08"]
    },
    hardhat: {
      mining: {
        auto: false,
        interval: [3000, 6000]
      },
      timeout: 2000000,
      accounts: [
        {
          privateKey:
            "0xbbfb76c92cd13796899f63dc6ead6d2420e8d0bc502d42bd5773c2d4b8897f08",
          balance: "4951760157141521099596496895000000"
        }
      ]
    }
  },
  typechain: {
    outDir: "typechain",
    target: "ethers-v5",
    runOnCompile: true
  },
  gasReporter: {
    enabled: true
  },
  mocha: {
    timeout: 2000000
  }
};
