use clarity::address::Address as EthAddress;
use clarity::Uint256;
use futures::join;
use web30::amm::DAI_CONTRACT_ADDRESS;
use web30::amm::USDC_CONTRACT_ADDRESS;
use web30::amm::USDT_CONTRACT_ADDRESS;
use web30::amm::WETH_CONTRACT_ADDRESS;
use web30::client::Web3;
use web30::jsonrpc::error::Web3Error;

const FIVE_PERCENT: f64 = 0.05f64; // used as an acceptable amount of slippage

/// First fetches the amount of WETH obtainable for `amount` of `token` from all the Uniswap v2 and v3 token/WETH pools,
/// potentially fetching the stablecoin intermediary (token -> USDC/USDT/DAI -> WETH) paths if no suitable WETH pairing exists.
/// Stablecoin prices are preferred in USDC, USDT, DAI order
pub async fn get_weth_price_with_retries(
    pubkey: EthAddress,
    token: EthAddress,
    amount: Uint256,
    web3: &Web3,
) -> Result<Uint256, Web3Error> {
    if token == *WETH_CONTRACT_ADDRESS {
        return Ok(amount);
    } else if amount == 0u8.into() {
        return Ok(0u8.into());
    }

    // Direct swap attempt via Uniswap v3 contracts
    let v3_weth = web3.get_uniswap_v3_price_with_retries(
        pubkey,
        token,
        *WETH_CONTRACT_ADDRESS,
        amount,
        Some(FIVE_PERCENT),
        None,
    );
    // Direct swap attempt via Uniswap v2 contracts
    let v2_weth = web3.get_uniswap_v2_price(pubkey, token, *WETH_CONTRACT_ADDRESS, amount, None);

    // If either of those is successful, return early (preferring v3)
    match join!(v3_weth, v2_weth) {
        (Ok(weth), _) => {
            println!("Got weth directly from Uniswap v3: {weth:?}");
            Ok(weth)
        }
        (Err(_), Ok(weth)) => {
            println!("Got weth directly from Uniswap v2: {weth:?}");
            Ok(weth)
        }
        // Otherwise, try to get the price
        (_, _) => {
            // Attempt to get the swap price with 1 stable intermediary
            println!("Failed to get a weth price directly, trying stable swaps");
            let StablePrices { usdc, usdt, dai } =
                get_stable_prices(pubkey, token, amount, web3).await?;
            println!("Stable swap results: {usdc:?} {usdt:?} {dai:?}");

            if let Ok(amt) = usdc {
                return get_weth_price(pubkey, *USDC_CONTRACT_ADDRESS, amt, web3).await;
            }
            if let Ok(amt) = usdt {
                return get_weth_price(pubkey, *USDT_CONTRACT_ADDRESS, amt, web3).await;
            }
            if let Ok(amt) = dai {
                return get_weth_price(pubkey, *DAI_CONTRACT_ADDRESS, amt, web3).await;
            }

            Err(Web3Error::BadResponse(
                "Failed to get a price from Uniswap".to_string(),
            ))
        }
    }
}

/// The amount of the given stable coins obtainable in a Uniswap swap
#[derive(Debug)]
pub struct StablePrices {
    pub usdc: Result<Uint256, Web3Error>,
    pub usdt: Result<Uint256, Web3Error>,
    pub dai: Result<Uint256, Web3Error>,
}

/// Fetches the amounts of USDC, USDT, and DAI obtainable for `amount` of `in_token` via Uniswap v3
pub async fn get_stable_prices(
    pubkey: EthAddress,
    in_token: EthAddress,
    amount: Uint256,
    web3: &Web3,
) -> Result<StablePrices, Web3Error> {
    let usdc_fut = web3.get_uniswap_v3_price_with_retries(
        pubkey,
        in_token,
        *USDC_CONTRACT_ADDRESS,
        amount,
        Some(FIVE_PERCENT),
        None,
    );
    let usdt_fut = web3.get_uniswap_v3_price_with_retries(
        pubkey,
        in_token,
        *USDT_CONTRACT_ADDRESS,
        amount,
        Some(FIVE_PERCENT),
        None,
    );
    let dai_fut = web3.get_uniswap_v3_price_with_retries(
        pubkey,
        in_token,
        *DAI_CONTRACT_ADDRESS,
        amount,
        Some(FIVE_PERCENT),
        None,
    );
    let (usdc, usdt, dai) = join!(usdc_fut, usdt_fut, dai_fut);
    Ok(StablePrices { usdc, usdt, dai })
}

/// utility function, gets the price of a given ERC20 token in uniswap in WETH given the erc20 address and amount
/// note that this function will only query the default fee pool (0.3%)
pub async fn get_weth_price(
    pubkey: EthAddress,
    token: EthAddress,
    amount: Uint256,
    web3: &Web3,
) -> Result<Uint256, Web3Error> {
    if token == *WETH_CONTRACT_ADDRESS {
        return Ok(amount);
    } else if amount == 0u8.into() {
        return Ok(0u8.into());
    }

    let slippage_sqrt_price = web3
        .get_v3_slippage_sqrt_price(pubkey, token, *WETH_CONTRACT_ADDRESS, None, FIVE_PERCENT)
        .await?;
    let price = web3
        .get_uniswap_v3_price(
            pubkey,
            token,
            *WETH_CONTRACT_ADDRESS,
            None,
            amount,
            Some(slippage_sqrt_price),
            None,
        )
        .await;
    if price.is_ok() {
        info!("WETH price for {amount} of {token}: {price:?}");
        return price;
    }
    info!("Uniswap v3 call failed, getting price from v2");
    web3.get_uniswap_v2_price(pubkey, token, *WETH_CONTRACT_ADDRESS, amount, None)
        .await
}

/// utility function, gets the price of a given ER20 token in uniswap in DAI given the erc20 address and amount
/// note that this function will only query the default fee pool (0.3%)
pub async fn get_dai_price(
    pubkey: EthAddress,
    token: EthAddress,
    amount: Uint256,
    web3: &Web3,
) -> Result<Uint256, Web3Error> {
    if token == *DAI_CONTRACT_ADDRESS {
        return Ok(amount);
    } else if amount == 0u8.into() {
        return Ok(0u8.into());
    }

    let slippage_sqrt_price = web3
        .get_v3_slippage_sqrt_price(pubkey, token, *DAI_CONTRACT_ADDRESS, None, FIVE_PERCENT)
        .await?;
    let price = web3
        .get_uniswap_v3_price(
            pubkey,
            token,
            *DAI_CONTRACT_ADDRESS,
            None,
            amount,
            Some(slippage_sqrt_price),
            None,
        )
        .await;
    if price.is_ok() {
        return price;
    }
    web3.get_uniswap_v2_price(pubkey, token, *DAI_CONTRACT_ADDRESS, amount, None)
        .await
}

#[cfg(test)]
mod tests {
    use actix::System;
    use clarity::Address;
    use clarity::Uint256;
    use env_logger;
    use std::time::Duration;
    use web30::client::Web3;

    use super::get_weth_price_with_retries;

    #[test]
    #[ignore]
    fn test_cheqd_price() {
        use env_logger::{Builder, Env};
        Builder::from_env(Env::default().default_filter_or("info,web30=debug")).init();
        let runner = System::new();
        let web3 = Web3::new("https://eth.althea.net", Duration::from_secs(30));
        let caller_address =
            Address::parse_and_validate("0x00000000219ab540356cBB839Cbe05303d7705Fa").unwrap();
        let one_cheqd = Uint256::from(1_000_000_000u64); // 10^9 1 cheqd token
        let cheqd =
            Address::parse_and_validate("0x70EDF1c215D0ce69E7F16FD4E6276ba0d99d4de7").unwrap();

        runner.block_on(async {
            let price = get_weth_price_with_retries(caller_address, cheqd, one_cheqd, &web3).await;
            println!("Got price in WETH: {price:?}");
        });
        panic!("Error for logs");
    }
}
