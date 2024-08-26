
/// Tests the auth functionality of the icaauth module on IBC test chain by trying to submit messages for an unowned interchain account
/// 1. Enable Host on Gravity and Controller on the IBC test chain
/// 2. Register two Interchain Accounts controlled by the IBC test chain and fund one with footoken
/// 3. Deploy an ERC20 for footoken on Ethereum
/// 4. Submit a MsgSendToEth to the ICA Controller module for the wrong account (owner 1 for ica 0)
pub async fn ica_hijack(
    web30: &Web3,
    grpc_client: GravityQueryClient<Channel>,
    gravity_contact: &Contact,
    ibc_contact: &Contact,
    keys: Vec<ValidatorKeys>,
    ibc_keys: Vec<CosmosPrivateKey>,
    gravity_address: EthAddress,
) {
    let mut grpc_client = grpc_client;
    let no_relay_market_config = create_default_test_config();
    start_orchestrators(keys.clone(), gravity_address, false, no_relay_market_config).await;

    let gravity_channel_qc = IbcChannelQueryClient::connect(COSMOS_NODE_GRPC.as_str())
        .await
        .expect("Could not connect channel query client");
    let ica_controller_qc = ICAControllerQueryClient::connect(IBC_NODE_GRPC.as_str())
        .await
        .expect("Cound not connect ica controller query client");

    // Wait for the ibc channel to be created and find the channel ids
    let channel_id_timeout = Duration::from_secs(60 * 5);
    let gravity_channel = get_channel(
        gravity_channel_qc,
        get_ibc_chain_id(),
        Some(channel_id_timeout),
    )
    .await
    .expect("Could not find gravity-test-1 channel");
    let gravity_connection_id = gravity_channel.connection_hops[0].clone();

    info!("\n\n!!!!!!!!!! Start ICA Hijack Test !!!!!!!!!!\n\n");
    enable_ica_host(
        gravity_contact,
        &keys,
    )
    .await;
    enable_ica_controller(
        &ibc_contact,
        &keys,
    )
    .await;

    let zero_fee = Coin{ amount: 0u8.into(), denom: STAKING_TOKEN.to_string() };

    let ica0_owner = ibc_keys[0].clone();
    let ica0_owner_addr = ica0_owner.to_address(&IBC_ADDRESS_PREFIX).unwrap();
    let ica0_addr: String = get_or_register_ica(&ibc_contact, ica_controller_qc.clone(), ica0_owner, ica0_owner_addr.to_string(), gravity_connection_id.clone(), zero_fee.clone()).await.expect("Could not get/register interchain account");
    let ica0_address = Address::from_bech32(ica0_addr).expect("invalid interchain account address?");

    sleep(Duration::from_secs(15)).await;

    let ica1_owner = ibc_keys[1].clone();
    let ica1_owner_addr = ica1_owner.to_address(&IBC_ADDRESS_PREFIX).unwrap();
    let ica1_addr: String = get_or_register_ica(&ibc_contact, ica_controller_qc, ica1_owner, ica1_owner_addr.to_string(), gravity_connection_id.clone(), zero_fee.clone()).await.expect("Could not get/register interchain account");
    let _ica1_address = Address::from_bech32(ica1_addr).expect("invalid interchain account address?");

    info!("Funding victim interchain account");
    let footoken = footoken_metadata(gravity_contact).await;
    let some_stake = Coin{amount: one_atom(), denom: STAKING_TOKEN.to_string()};
    let some_foo = Coin{amount: one_atom() * 5u8.into(), denom: footoken.base.clone()};
    gravity_contact.send_coins(
        some_stake,
        Some(zero_fee.clone()),
        ica0_address,
        Some(OPERATION_TIMEOUT),
        keys[0].validator_key.clone(),
    ).await.expect("Failed to fund victim ICA");
    gravity_contact.send_coins(
        some_foo,
        Some(zero_fee),
        ica0_address,
        Some(OPERATION_TIMEOUT),
        keys[0].validator_key.clone(),
    ).await.expect("Failed to fund victim ICA");

    let erc20_contract = deploy_cosmos_representing_erc20_and_check_adoption(
        gravity_address,
        web30,
        Some(keys.clone()),
        &mut grpc_client,
        false,
        footoken.clone(),
    )
    .await;
    let token_to_send_to_eth = footoken.base;
    let amount_to_bridge: Uint256 = one_atom();
    let chain_fee: Uint256 = 500u64.into(); // A typical chain fee is 2 basis points, this gives us a bit of wiggle room
    let send_to_eth_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: amount_to_bridge,
    };
    let chain_fee_coin = Coin {
        denom: token_to_send_to_eth.clone(),
        amount: chain_fee,
    };
    let bridge_fee_coin = get_fee(Some(token_to_send_to_eth.clone()));

    // Attempt the hijack
    let eth_receiver = keys[2].eth_key.to_address();
    let starting_balance = get_erc20_balance_safe(erc20_contract, web30, eth_receiver)
        .await
        .unwrap();
    let res = send_to_eth_via_ica(
        &ibc_contact,
        gravity_contact,
        ica0_address,
        ica1_owner,
        ica1_owner_addr,
        gravity_connection_id,
        eth_receiver,
        send_to_eth_coin,
        bridge_fee_coin,
        Some(chain_fee_coin),
    )
    .await;
    info!("Got result {res:?}");

    sleep(OPERATION_TIMEOUT).await;

    let ending_balance = get_erc20_balance_safe(erc20_contract, web30, eth_receiver)
        .await
        .unwrap();

    info!("start: {starting_balance}, end: {ending_balance}");
}