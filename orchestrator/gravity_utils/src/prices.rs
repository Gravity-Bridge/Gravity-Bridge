use clarity::Uint256;
use web30::amm::DAI_CONTRACT_ADDRESS;
use web30::amm::WETH_CONTRACT_ADDRESS;
use web30::client::Web3;
use web30::jsonrpc::error::Web3Error;
use web30::EthAddress;

const FIVE_PERCENT: f64 = 0.05f64; // used as an acceptable amount of slippage

/// utility function, iteratively queries the standard Uniswap v3 pools to find a price in WETH,
/// if that fails then we query uniswap v2
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

    // First try to use uniswap v3
    let res = web3
        .get_uniswap_v3_price_with_retries(
            pubkey,
            token,
            *WETH_CONTRACT_ADDRESS,
            amount.clone(),
            Some(FIVE_PERCENT),
            None,
        )
        .await;
    if res.is_ok() {
        return res;
    }
    // If that fails, use uniswap v2
    web3.get_uniswap_v2_price(pubkey, token, *WETH_CONTRACT_ADDRESS, amount, None)
        .await
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
            amount.clone(),
            Some(slippage_sqrt_price),
            None,
        )
        .await;
    if price.is_ok() {
        return price;
    }
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
            amount.clone(),
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
