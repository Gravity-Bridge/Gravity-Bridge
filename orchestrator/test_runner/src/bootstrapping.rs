use core::str::FromStr;
use std::thread;

use crate::get_deposit;
use crate::HERMES_CONFIG;
use crate::MINER_PRIVATE_KEY;
use crate::OPERATION_TIMEOUT;
use crate::RELAYER_ADDRESS;
use crate::RELAYER_MNEMONIC;
use crate::TOTAL_TIMEOUT;
use crate::{get_gravity_chain_id, get_ibc_chain_id, ETH_NODE};
use crate::{utils::ValidatorKeys, COSMOS_NODE_ABCI};
use clarity::PrivateKey as EthPrivateKey;
use deep_space::private_key::{CosmosPrivateKey, PrivateKey, DEFAULT_COSMOS_HD_PATH};
use deep_space::Contact;
use ibc::core::ics24_host::identifier::ChainId;
use ibc_relayer::config::AddressType;
use ibc_relayer::keyring::{HDPath, KeyRing, Store};
use std::os::unix::io::{FromRawFd, IntoRawFd};
use std::process::{Command, Stdio};
use std::{fs::File, path::Path};
use std::{
    io::{BufRead, BufReader, Read, Write},
    process::ExitStatus,
};
use web30::EthAddress;

/// Ethereum private keys for the validators are generated using the gravity eth_keys add command
/// and dumped into a file /validator-eth-keys in the container, from there they are then used by
/// the orchestrator on startup
pub fn parse_ethereum_keys() -> Vec<EthPrivateKey> {
    let filename = "/validator-eth-keys";
    let file = File::open(filename).expect("Failed to find eth keys");
    let reader = BufReader::new(file);
    let mut ret = Vec::new();

    for line in reader.lines() {
        let key = line.expect("Error reading eth key file!");
        if key.is_empty() || key.contains("public") || key.contains("address") {
            continue;
        }
        let key = key.split(':').last().unwrap().trim();
        ret.push(key.parse().unwrap());
    }
    ret
}

/// Parses the output of the cosmoscli keys add command to import the private key
fn parse_phrases(filename: &str) -> (Vec<CosmosPrivateKey>, Vec<String>) {
    let file = File::open(filename).expect("Failed to find phrases");
    let reader = BufReader::new(file);
    let mut ret_keys = Vec::new();
    let mut ret_phrases = Vec::new();

    for line in reader.lines() {
        let phrase = line.expect("Error reading phrase file!");
        if phrase.is_empty()
            || phrase.contains("write this mnemonic phrase")
            || phrase.contains("recover your account if")
        {
            continue;
        }
        let key = CosmosPrivateKey::from_phrase(&phrase, "").expect("Bad phrase!");
        ret_keys.push(key);
        ret_phrases.push(phrase);
    }
    (ret_keys, ret_phrases)
}

/// Validator private keys are generated via the gravity key add
/// command, from there they are used to create gentx's and start the
/// chain, these keys change every time the container is restarted.
/// The mnemonic phrases are dumped into a text file /validator-phrases
/// the phrases are in increasing order, so validator 1 is the first key
/// and so on. While validators may later fail to start it is guaranteed
/// that we have one key for each validator in this file.
pub fn parse_validator_keys() -> (Vec<CosmosPrivateKey>, Vec<String>) {
    let filename = "/validator-phrases";
    info!("Reading mnemonics from {}", filename);
    parse_phrases(filename)
}

/// The same as parse_validator_keys() except for a second chain accessed
/// over IBC for testing purposes
pub fn parse_ibc_validator_keys() -> (Vec<CosmosPrivateKey>, Vec<String>) {
    let filename = "/ibc-validator-phrases";
    info!("Reading mnemonics from {}", filename);
    parse_phrases(filename)
}

/// Orchestrator private keys are generated via the gravity key add
/// command just like the validator keys themselves and stored in a
/// similar file /orchestrator-phrases
pub fn parse_orchestrator_keys() -> Vec<CosmosPrivateKey> {
    let filename = "/orchestrator-phrases";
    info!("Reading orchestrator phrases from {}", filename);
    let (orch_keys, _) = parse_phrases(filename);
    orch_keys
}

/// Vesting private keys are generated via the gravity key add
/// command just like the validator keys themselves and stored in a
/// similar file /vesting-phrases
pub fn parse_vesting_keys() -> Vec<CosmosPrivateKey> {
    let filename = "/vesting-phrases";
    info!("Reading vesting phrases from {}", filename);
    let (vest_keys, _) = parse_phrases(filename);
    vest_keys
}

pub fn get_keys() -> Vec<ValidatorKeys> {
    let (cosmos_keys, cosmos_phrases) = parse_validator_keys();
    let orch_keys = parse_orchestrator_keys();
    let eth_keys = parse_ethereum_keys();
    let mut ret = Vec::new();
    for (((c_key, c_phrase), o_key), e_key) in cosmos_keys
        .into_iter()
        .zip(cosmos_phrases)
        .zip(orch_keys)
        .zip(eth_keys)
    {
        ret.push(ValidatorKeys {
            eth_key: e_key,
            validator_key: c_key,
            validator_phrase: c_phrase,
            orch_key: o_key,
        })
    }
    ret
}

/// This function deploys the required contracts onto the Ethereum testnet
/// this runs only when the DEPLOY_CONTRACTS env var is set right after
/// the Ethereum test chain starts in the testing environment. We write
/// the stdout of this to a file for later test runs to parse
pub async fn deploy_contracts(contact: &Contact) {
    // prevents the node deployer from failing (rarely) when the chain has not
    // yet produced the next block after submitting each eth address
    contact.wait_for_next_block(TOTAL_TIMEOUT).await.unwrap();

    // these are the possible paths where we could find the contract deployer
    // and the gravity contract itself, feel free to expand this if it makes your
    // deployments more straightforward.

    // both files are just in the PWD
    const A: [&str; 3] = ["contract-deployer", "Gravity.json", "GravityERC721.json"];
    // files are placed in a root /solidity/ folder
    const B: [&str; 3] = [
        "/solidity/contract-deployer",
        "/solidity/Gravity.json",
        "/solidity/GravityERC721.json",
    ];
    // the default unmoved locations for the Gravity repo
    const C: [&str; 4] = [
        "/gravity/solidity/contract-deployer.ts",
        "/gravity/solidity/artifacts/contracts/Gravity.sol/Gravity.json",
        "/gravity/solidity/artifacts/contracts/GravityERC721.sol/GravityERC721.json",
        "/gravity/solidity/",
    ];
    let output = if all_paths_exist(&A) || all_paths_exist(&B) {
        let paths = return_existing(A, B);
        Command::new(paths[0])
            .args([
                &format!("--cosmos-node={}", COSMOS_NODE_ABCI.as_str()),
                &format!("--eth-node={}", ETH_NODE.as_str()),
                &format!("--eth-privkey={:#x}", *MINER_PRIVATE_KEY),
                &format!("--contract={}", paths[1]),
                &format!("--contractERC721={}", paths[2]),
                "--test-mode=true",
            ])
            .output()
            .expect("Failed to deploy contracts!")
    } else if all_paths_exist(&C) {
        Command::new("npx")
            .args([
                "ts-node",
                C[0],
                &format!("--cosmos-node={}", COSMOS_NODE_ABCI.as_str()),
                &format!("--eth-node={}", ETH_NODE.as_str()),
                &format!("--eth-privkey={:#x}", *MINER_PRIVATE_KEY),
                &format!("--contract={}", C[1]),
                &format!("--contractERC721={}", C[2]),
                "--test-mode=true",
            ])
            .current_dir(C[3])
            .output()
            .expect("Failed to deploy contracts!")
    } else {
        panic!("Could not find Gravity.json contract artifact in any known location!")
    };

    info!("stdout: {}", String::from_utf8_lossy(&output.stdout));
    info!("stderr: {}", String::from_utf8_lossy(&output.stderr));
    if !ExitStatus::success(&output.status) {
        panic!("Contract deploy failed!")
    }
    let mut file = File::create("/contracts").unwrap();
    file.write_all(&output.stdout).unwrap();
}

pub struct BootstrapContractAddresses {
    pub gravity_contract: EthAddress,
    pub gravity_erc721_contract: EthAddress,
    pub erc20_addresses: Vec<EthAddress>,
    pub erc721_addresses: Vec<EthAddress>,
    pub uniswap_liquidity_address: Option<EthAddress>,
}

/// Parses the ERC20 and Gravity contract addresses from the file created
/// in deploy_contracts()
pub fn parse_contract_addresses() -> BootstrapContractAddresses {
    let mut file =
        File::open("/contracts").expect("Failed to find contracts! did they not deploy?");
    let mut output = String::new();
    file.read_to_string(&mut output).unwrap();
    let mut maybe_gravity_address = None;
    let mut maybe_gravity_erc721_address = None;
    let mut erc20_addresses = Vec::new();
    let mut erc721_addresses = Vec::new();
    let mut uniswap_liquidity = None;
    for line in output.lines() {
        if line.contains("Gravity deployed at Address -") {
            let address_string = line.split('-').last().unwrap();
            maybe_gravity_address = Some(address_string.trim().parse().unwrap());
        } else if line.contains("GravityERC721 deployed at Address -") {
            let address_string = line.split('-').last().unwrap();
            maybe_gravity_erc721_address = Some(address_string.trim().parse().unwrap());
        } else if line.contains("ERC20 deployed at Address -") {
            let address_string = line.split('-').last().unwrap();
            erc20_addresses.push(address_string.trim().parse().unwrap());
            info!("found erc20 address it is {}", address_string);
        } else if line.contains("ERC721 deployed at Address -") {
            let address_string = line.split('-').last().unwrap();
            erc721_addresses.push(address_string.trim().parse().unwrap());
            info!("found erc721 address it is {}", address_string);
        } else if line.contains("Uniswap Liquidity test deployed at Address - ") {
            let address_string = line.split('-').last().unwrap();
            uniswap_liquidity = Some(address_string.trim().parse().unwrap());
        }
    }
    let gravity_address: EthAddress = maybe_gravity_address.unwrap();
    let gravity_erc721_address: EthAddress = maybe_gravity_erc721_address.unwrap();
    BootstrapContractAddresses {
        gravity_contract: gravity_address,
        gravity_erc721_contract: gravity_erc721_address,
        erc20_addresses,
        erc721_addresses,
        uniswap_liquidity_address: uniswap_liquidity,
    }
}

fn all_paths_exist(input: &[&str]) -> bool {
    for i in input {
        if !Path::new(i).exists() {
            return false;
        }
    }
    true
}

fn return_existing<'a>(a: [&'a str; 3], b: [&'a str; 3]) -> [&'a str; 3] {
    if all_paths_exist(&a) {
        a
    } else if all_paths_exist(&b) {
        b
    } else {
        panic!("No paths exist!")
    }
}

// Creates a key in the relayer's test keyring, which the relayer should use
// Hermes stores its keys in hermes_home/ gravity_phrase is for the main chain
/// ibc phrase is for the test chain
pub fn setup_relayer_keys(
    gravity_phrase: &str,
    ibc_phrase: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let mut keyring = KeyRing::new(
        Store::Test,
        "gravity",
        &ChainId::from_string(&get_gravity_chain_id()),
    )?;

    let key = keyring.key_from_mnemonic(
        gravity_phrase,
        &HDPath::from_str(DEFAULT_COSMOS_HD_PATH).unwrap(),
        &AddressType::Cosmos,
    )?;
    keyring.add_key("gravitykey", key)?;

    keyring = KeyRing::new(
        Store::Test,
        "cosmos",
        &ChainId::from_string(&get_ibc_chain_id()),
    )?;
    let key = keyring.key_from_mnemonic(
        ibc_phrase,
        &HDPath::from_str(DEFAULT_COSMOS_HD_PATH).unwrap(),
        &AddressType::Cosmos,
    )?;
    keyring.add_key("ibckey", key)?;

    Ok(())
}

// Create a channel between gravity chain and the ibc test chain over the "transfer" port
// Writes the output to /ibc-relayer-logs/channel-creation
pub fn create_ibc_channel(hermes_base: &mut Command) {
    // hermes -c config.toml create channel gravity-test-1 ibc-test-1 --port-a transfer --port-b transfer
    let create_channel = hermes_base.args([
        "create",
        "channel",
        &get_gravity_chain_id(),
        &get_ibc_chain_id(),
        "--port-a",
        "transfer",
        "--port-b",
        "transfer",
    ]);

    let out_file = File::options()
        .write(true)
        .open("/ibc-relayer-logs/channel-creation")
        .unwrap()
        .into_raw_fd();
    unsafe {
        // unsafe needed for stdout + stderr redirect to file
        let create_channel = create_channel
            .stdout(Stdio::from_raw_fd(out_file))
            .stderr(Stdio::from_raw_fd(out_file));
        info!("Create channel command: {:?}", create_channel);
        create_channel.spawn().expect("Could not create channel");
    }
}

// Start an IBC relayer locally and run until it terminates
// full_scan Force a full scan of the chains for clients, connections and channels
// Writes the output to /ibc-relayer-logs/hermes-logs
pub fn run_ibc_relayer(hermes_base: &mut Command, full_scan: bool) {
    let mut start = hermes_base.arg("start");
    if full_scan {
        start = start.arg("-f");
    }
    let out_file = File::options()
        .write(true)
        .open("/ibc-relayer-logs/hermes-logs")
        .unwrap()
        .into_raw_fd();
    unsafe {
        // unsafe needed for stdout + stderr redirect to file
        start
            .stdout(Stdio::from_raw_fd(out_file))
            .stderr(Stdio::from_raw_fd(out_file))
            .spawn()
            .expect("Could not run hermes");
    }
}

// starts up the IBC relayer (hermes) in a background thread
pub async fn start_ibc_relayer(contact: &Contact, keys: &[ValidatorKeys], ibc_phrases: &[String]) {
    contact
        .send_coins(
            get_deposit(),
            None,
            *RELAYER_ADDRESS,
            Some(OPERATION_TIMEOUT),
            keys[0].validator_key,
        )
        .await
        .unwrap();
    info!("test-runner starting IBC relayer mode: init hermes, create ibc channel, start hermes");
    let mut hermes_base = Command::new("hermes");
    let hermes_base = hermes_base.arg("-c").arg(HERMES_CONFIG);
    setup_relayer_keys(&RELAYER_MNEMONIC, &ibc_phrases[0]).unwrap();
    create_ibc_channel(hermes_base);
    thread::spawn(|| {
        let mut hermes_base = Command::new("hermes");
        let hermes_base = hermes_base.arg("-c").arg(HERMES_CONFIG);
        run_ibc_relayer(hermes_base, true); // likely will not return from here, just keep running
    });
    info!("Running ibc relayer in the background, directing output to /ibc-relayer-logs");
}
