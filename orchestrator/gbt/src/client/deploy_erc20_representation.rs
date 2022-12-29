use crate::{args::DeployErc20RepresentationOpts, utils::TIMEOUT};
use clarity::{utils::display_uint256_as_address, Address as EthAddress};
use cosmos_gravity::query::get_gravity_params;
use ethereum_gravity::deploy_erc20::deploy_erc20;
use gravity_proto::gravity::{
    MsgErc20DeployedClaim, QueryAttestationsRequest, QueryDenomToErc20Request,
};
use gravity_utils::connection_prep::{check_for_eth, create_rpc_connections};
use prost::{bytes::BytesMut, Message};
use std::{
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
    let evm_chain_prefix = args.evm_chain_prefix;

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
            evm_chain_prefix: evm_chain_prefix.to_string(),
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
            info!("Retrieved metadata starting deploy of ERC20");
            let mut decimals = None;
            for unit in metadata.denom_units {
                if unit.denom == metadata.display {
                    decimals = Some(unit.exponent)
                }
            }
            let decimals = decimals.unwrap();
            let contract_to_be_adopted = deploy_erc20(
                metadata.base,
                metadata.name,
                metadata.symbol,
                decimals,
                contract_address,
                &web3,
                Some(TIMEOUT),
                ethereum_key,
                vec![SendTxOption::GasPriceMultiplier(1.5)],
            )
            .await
            .unwrap();
            // converts uint256 contract call response into an ETH address
            let contract_to_be_adopted: EthAddress =
                display_uint256_as_address(contract_to_be_adopted)
                    .parse()
                    .unwrap();

            info!("We have deployed ERC20 contract {}, waiting to see if the Cosmos chain choses to adopt it", contract_to_be_adopted);

            let start = Instant::now();
            loop {
                let res = grpc
                    .denom_to_erc20(QueryDenomToErc20Request {
                        evm_chain_prefix: evm_chain_prefix.to_string(),
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

                // we wait for up to WAIT_TIME seconds after that we must investigate why the attestation failed
                const WAIT_TIME: u64 = 600;
                if Instant::now() - start > Duration::from_secs(WAIT_TIME) {
                    let attestations = grpc
                        .get_attestations(QueryAttestationsRequest {
                            limit: 0,
                            order_by: String::new(),
                            claim_type: String::new(),
                            nonce: 0,
                            height: 0,
                            use_v1_key: false,
                            evm_chain_prefix: evm_chain_prefix.to_string(),
                        })
                        .await;
                    match attestations {
                        Ok(attestations) => {
                            let attestations = attestations.into_inner().attestations;
                            for a in attestations {
                                // the else condition here should never happen as it would mean we have an event with a nil pointer
                                if let Some(claim) = a.claim {
                                    if claim.type_url.contains("MsgERC20DeployedClaim") {
                                        // decode any value to get at the actual contents of this claim
                                        let mut buf = BytesMut::with_capacity(claim.value.len());
                                        buf.extend_from_slice(&claim.value);
                                        let claim_contents = MsgErc20DeployedClaim::decode(buf)
                                            .expect("Failed to decode claim");

                                        let claim_contract: EthAddress =
                                            claim_contents.token_contract.parse().unwrap();
                                        if claim_contract == contract_to_be_adopted {
                                            if a.observed {
                                                error!("Your ERC20 contract has been rejected by the Gravity Bridge chain, please check the metadata and try again");
                                                exit(1);
                                            } else {
                                                error!("Validators have not finished processing this deployment event after {} seconds", WAIT_TIME);
                                                error!("At this time your ERC20 contract may or may not have been adopted by the bridge, you will have to confirm either by checking the erc20_to_denom field of a genesis dump or using the denom_to_erc20 query endpoint.");
                                                exit(1);
                                            }
                                        }
                                    }
                                }
                            }
                            error!("We were unable to find your ERC20 as a claim after {} seconds. Are you sure the Ethereum transaction went through? Is the node you are using keeping up with the chain?", WAIT_TIME);
                            exit(1);
                        }
                        Err(e) => {
                            error!("After waiting {} seconds the ERC20 contract was not adopted, when attempting to check why the adoption failed we encountered an error {:?}", WAIT_TIME, e);
                            error!("At this time your ERC20 contract may or may not have been adopted by the bridge, you will have to confirm either by checking the erc20_to_denom field of a genesis dump or using the denom_to_erc20 query endpoint.");
                            exit(1);
                        }
                    }
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

#[cfg(test)]
mod tests {
    use super::*;
    use gravity_proto::gravity::query_client::QueryClient as GravityQueryClient;

    // Check that our attestation querying and decoding will work, this is hand test and should probably
    // be turned into a reusable function in gravity_utils
    #[actix_rt::test]
    #[ignore]
    async fn test_endpoints() {
        let mut grpc = GravityQueryClient::connect("https://gravitychain.io:9090")
            .await
            .unwrap();
        let attestations = grpc
            .get_attestations(QueryAttestationsRequest {
                limit: 1000,
                order_by: String::new(),
                claim_type: String::new(),
                nonce: 0,
                height: 0,
                use_v1_key: false,
                evm_chain_prefix: "bsc".to_string(),
            })
            .await
            .unwrap();
        let attestations = attestations.into_inner().attestations;
        assert!(!attestations.is_empty());
        for a in attestations {
            // the else condition here should never happen as it would mean we have an event with a nil pointer
            if let Some(claim) = a.claim {
                // required because claim type filtering does not seem to be working as expected
                if claim.type_url.contains("MsgERC20DeployedClaim") {
                    // decode any value to get at the actual contents of this claim
                    let mut buf = BytesMut::with_capacity(claim.value.len());
                    buf.extend_from_slice(&claim.value);
                    let claim_contents =
                        MsgErc20DeployedClaim::decode(buf).expect("Failed to decode claim");
                    println!("Got claim {:?}", claim_contents);
                }
            }
        }
    }
}
