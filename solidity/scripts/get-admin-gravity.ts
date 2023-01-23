import Web3 from 'web3';
import gravityArtifacts from '../artifacts/contracts/Gravity.sol/Gravity.json';
const web3 = new Web3(process.env.JSON_RPC || 'http://localhost:8545');

function getContract() {
  return new web3.eth.Contract(gravityArtifacts.abi as any, process.env.GRAVITY_CONTRACT);
}

async function queryGravityAdmin() {
  const gravityContract = getContract();
  const result = await gravityContract.methods.getAdminAddress().call();
  console.log("result: ", result)
}

queryGravityAdmin();