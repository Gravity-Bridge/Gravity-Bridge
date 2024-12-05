#[macro_use]
extern crate log;
#[macro_use]
extern crate serde_derive;

use crate::args::{ClientSubcommand, KeysSubcommand, SubCommand};
use crate::config::init_config;
use crate::keys::{recover_funds, show_keys};
use crate::{jsonrpc_server::jsonrpc_server, orchestrator::orchestrator, relayer::relayer};
use args::{GovQuerySubcommand, GovSubcommand, GovSubmitSubcommand, Opts};
use clap::Parser;
use client::cosmos_to_eth::cosmos_to_eth_cmd;
use client::deploy_erc20_representation::deploy_erc20_representation;
use client::eth_to_cosmos::eth_to_cosmos;
use client::request_all_batches::request_all_batches;
use client::spot_relay::spot_relay;
use config::{get_home_dir, load_config};
use env_logger::Env;
use gov::proposals::{
    submit_airdrop, submit_emergency_bridge_halt, submit_ibc_metadata, submit_oracle_unhalt,
};
use gov::queries::query_airdrops;
use keys::register_orchestrator_address::register_orchestrator_address;
use keys::set_eth_key;
use keys::set_orchestrator_key;
use rustls::crypto::aws_lc_rs;
use rustls::crypto::CryptoProvider;

mod args;
mod client;
mod config;
mod gov;
mod jsonrpc_server;
mod keys;
mod orchestrator;
mod relayer;
mod utils;

#[actix_rt::main]
async fn main() {
    let opts: Opts = Opts::parse();
    let log_level = match opts.verbose {
        true => "debug",
        false => "info",
    };
    env_logger::Builder::from_env(Env::default().default_filter_or(log_level)).init();
    // On Linux static builds we need to probe ssl certs path to be able to
    // do TLS stuff.
    openssl_probe::init_ssl_cert_env_vars();
    CryptoProvider::install_default(aws_lc_rs::default_provider()).unwrap();
    // parse the arguments

    // handle global config here
    let address_prefix = opts.address_prefix;
    let home_dir = get_home_dir(opts.home);
    let config = load_config(&home_dir);

    // control flow for the command structure
    match opts.subcmd {
        SubCommand::Client(client_opts) => match client_opts.subcmd {
            ClientSubcommand::EthToCosmos(eth_to_cosmos_opts) => {
                eth_to_cosmos(eth_to_cosmos_opts, address_prefix).await
            }
            ClientSubcommand::CosmosToEth(cosmos_to_eth_opts) => {
                cosmos_to_eth_cmd(cosmos_to_eth_opts, address_prefix).await
            }
            ClientSubcommand::DeployErc20Representation(deploy_erc20_opts) => {
                deploy_erc20_representation(deploy_erc20_opts, address_prefix).await
            }
            ClientSubcommand::SpotRelay(spot_relay_opts) => {
                spot_relay(spot_relay_opts, address_prefix).await
            }
            ClientSubcommand::RequestAllBatches(request_all_batches_opts) => {
                request_all_batches(request_all_batches_opts, address_prefix).await
            }
        },
        SubCommand::Keys(key_opts) => match key_opts.subcmd {
            KeysSubcommand::RegisterOrchestratorAddress(set_orchestrator_address_opts) => {
                register_orchestrator_address(
                    set_orchestrator_address_opts,
                    address_prefix,
                    home_dir,
                )
                .await
            }
            KeysSubcommand::Show => show_keys(&home_dir, &address_prefix),
            KeysSubcommand::SetEthereumKey(set_eth_key_opts) => {
                set_eth_key(&home_dir, set_eth_key_opts)
            }
            KeysSubcommand::SetOrchestratorKey(set_orch_key_opts) => {
                set_orchestrator_key(&home_dir, set_orch_key_opts)
            }
            KeysSubcommand::RecoverFunds(recover_funds_opts) => {
                recover_funds(recover_funds_opts, address_prefix).await
            }
        },
        SubCommand::Orchestrator(orchestrator_opts) => {
            orchestrator(orchestrator_opts, address_prefix, &home_dir, config).await
        }
        SubCommand::Relayer(relayer_opts) => {
            relayer(relayer_opts, address_prefix, &home_dir, config.relayer).await
        }
        SubCommand::JsonrpcServer(server_opts) => jsonrpc_server(server_opts).await,
        SubCommand::Init(init_opts) => init_config(init_opts, home_dir),
        SubCommand::Gov(gov_opts) => match gov_opts.subcmd {
            GovSubcommand::Submit(submit_opts) => match submit_opts {
                GovSubmitSubcommand::IbcMetadata(opts) => {
                    submit_ibc_metadata(opts, address_prefix).await
                }
                GovSubmitSubcommand::Airdrop(opts) => submit_airdrop(opts, address_prefix).await,
                GovSubmitSubcommand::EmergencyBridgeHalt(opts) => {
                    submit_emergency_bridge_halt(opts, address_prefix).await
                }
                GovSubmitSubcommand::OracleUnhalt(opts) => {
                    submit_oracle_unhalt(opts, address_prefix).await
                }
            },
            GovSubcommand::Query(query_opts) => match query_opts {
                GovQuerySubcommand::Airdrop(opts) => query_airdrops(opts, address_prefix).await,
            },
        },
    }
}
