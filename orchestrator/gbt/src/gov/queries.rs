use crate::args::AirdropQueryOpts;
use cosmos_gravity::send::TIMEOUT;
use deep_space::Address;
use gravity_proto::gravity::AirdropProposal;
use gravity_utils::connection_prep::create_rpc_connections;
use prost::{bytes::BytesMut, Message};
use std::process::exit;

pub async fn query_airdrops(opts: AirdropQueryOpts, prefix: String) {
    let connections =
        create_rpc_connections(prefix.clone(), Some(opts.cosmos_grpc), None, TIMEOUT).await;
    let contact = connections.contact.unwrap();

    info!("Getting details for active airdrop proposals");

    let proposals = if opts.query_history {
        contact.get_passed_governance_proposals().await
    } else {
        contact.get_governance_proposals_in_voting_period().await
    };

    let mut found = false;
    match proposals {
        Ok(proposals) => {
            for proposal in proposals.proposals {
                if let Some(content) = proposal.content {
                    let mut buf = BytesMut::with_capacity(content.value.len());
                    buf.extend_from_slice(&content.value);
                    let res = AirdropProposal::decode(buf);
                    if let Ok(airdrop) = res {
                        found = true;

                        info!("Found Airdrop proposal");
                        info!("Title: {}", airdrop.title);
                        info!("Description: {}", airdrop.description);
                        info!("Number of Participants: {}", airdrop.amounts.len());
                        let mut sum = 0;
                        for amount in airdrop.amounts.iter() {
                            sum += amount;
                        }
                        info!("Total value: {}{}", sum, airdrop.denom);

                        if airdrop.amounts.len() < 100 || opts.full_list {
                            info!("Participants list");

                            let mut unpacked = Vec::new();
                            // each address is 20 bytes
                            assert_eq!(airdrop.recipients.len() / 20, airdrop.amounts.len());
                            for i in 0..(airdrop.recipients.len() / 20) {
                                let mut buf = [0; 20];
                                let addr_bytes = &airdrop.recipients[(i * 20)..((i * 20) + 20)];
                                buf.copy_from_slice(addr_bytes);
                                let addr = Address::from_bytes(buf, prefix.clone()).unwrap();
                                unpacked.push(addr);
                            }
                            assert_eq!(unpacked.len(), airdrop.amounts.len());
                            for (i, _) in unpacked.iter().enumerate() {
                                info!("{} {}{}", unpacked[i], airdrop.amounts[i], airdrop.denom)
                            }
                        } else {
                            info!("Participants list is greater than 100 addresses, use --full-list to display it");
                        }
                    }
                }
            }
        }
        Err(e) => {
            error!("Failed to get proposals, check your cosmos gRPC {:?}", e);
            exit(1);
        }
    }
    if !found {
        info!("No Airdrop proposals meeting the criteria where found!")
    }
}
