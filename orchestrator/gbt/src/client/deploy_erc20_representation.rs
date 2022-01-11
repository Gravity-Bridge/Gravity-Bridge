use crate::{args::DeployErc20RepresentationOpts, utils::TIMEOUT};
use cosmos_gravity::query::get_gravity_params;
use ethereum_gravity::deploy_erc20::deploy_erc20;
use gravity_proto::gravity::QueryDenomToErc20Request;
use gravity_utils::connection_prep::{check_for_eth, create_rpc_connections};
use std::{
    convert::TryInto,
    process::exit,
    time::{Duration, Instant},
};
use tokio::time::sleep as delay_for;
use web30::types::SendTxOption;

pub async fn deploy_erc20_representation(
    args: DeployErc20RepresentationOpts,
    address_prefix: String,
) {
    let grpc_url = args.cosmos_grpc;
    let ethereum_rpc = args.ethereum_rpc;
    let ethereum_key = args.ethereum_key;
    let denom = args.cosmos_denom;

    let connections =
        create_rpc_connections(address_prefix, Some(grpc_url), Some(ethereum_rpc), TIMEOUT).await;
    let web3 = connections.web3.unwrap();
    let contact = connections.contact.unwrap();

    let mut grpc = connections.grpc.unwrap();

    let ethereum_public_key = ethereum_key.to_address();
    check_for_eth(ethereum_public_key, &web3).await;

    let contract_address = if let Some(c) = args.gravity_contract_address {
        c
    } else {
        let params = get_gravity_params(&mut grpc).await.unwrap();
        let c = params.bridge_ethereum_address.parse();
        if c.is_err() {
            error!("The Gravity address is not yet set as a chain parameter! You must specify --gravity-contract-address");
            exit(1);
        }
        c.unwrap()
    };

    let res = grpc
        .denom_to_erc20(QueryDenomToErc20Request {
            denom: denom.clone(),
        })
        .await;
    if let Ok(val) = res {
        info!(
            "Asset {} already has ERC20 representation {}",
            denom,
            val.into_inner().erc20
        );
        exit(1);
    }

    let res = contact.get_denom_metadata(denom.clone()).await;
    match res {
        Ok(Some(metadata)) => {
            info!("Retrieved metadta starting deploy of ERC20");
            let mut decimals = None;
            for unit in metadata.denom_units {
                if unit.denom == metadata.display {
                    decimals = Some(unit.exponent)
                }
            }
            let decimals = decimals.unwrap();
            let res = deploy_erc20(
                metadata.display,
                metadata.name,
                metadata.symbol,
                decimals.try_into().unwrap(),
                contract_address,
                &web3,
                Some(TIMEOUT),
                ethereum_key,
                vec![SendTxOption::GasPriceMultiplier(1.5)],
            )
            .await
            .unwrap();

            info!("We have deployed ERC20 contract {:#066x}, waiting to see if the Cosmos chain choses to adopt it", res);

            let start = Instant::now();
            loop {
                let res = grpc
                    .denom_to_erc20(QueryDenomToErc20Request {
                        denom: denom.clone(),
                    })
                    .await;

                if let Ok(val) = res {
                    info!(
                        "Asset {} has accepted new ERC20 representation {}",
                        denom,
                        val.into_inner().erc20
                    );
                    exit(0);
                }

                if Instant::now() - start > Duration::from_secs(100) {
                    info!("Your ERC20 contract was not adopted, double check the metadata and try again");
                    exit(1);
                }
                delay_for(Duration::from_secs(1)).await;
            }
        }
        Ok(None) => {
            warn!("denom {} has no denom metadata set, this means it is impossible to deploy an ERC20 representation at this time", denom);
            warn!("A governance proposal to set this denoms metadata will need to pass before running this command");
        }
        Err(e) => error!("Unable to make metadata request, check grpc {:?}", e),
    }
}
