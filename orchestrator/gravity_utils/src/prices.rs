use clarity::address::Address as EthAddress;
use clarity::Uint256;
use web30::amm::{DAI_CONTRACT_ADDRESS};
use web30::amm::WETH_CONTRACT_ADDRESS;
use web30::client::Web3;
use web30::jsonrpc::error::Web3Error;

const FIVE_PERCENT: f64 = 0.05f64;

/// utility function, iteratively queries the standard Uniswap v3 pools to find a price in WETH
pub async fn get_weth_price_with_retries(
    token: EthAddress,
    amount: Uint256,
    pubkey: EthAddress,
    web3: &Web3,
) -> Result<Uint256, Web3Error> {
    if token == *WETH_CONTRACT_ADDRESS {
        return Ok(amount);
    } else if amount == 0u8.into() {
        return Ok(0u8.into());
    }

    web3.get_uniswap_price_with_retries(*WETH_CONTRACT_ADDRESS, token, amount, None, pubkey).await
}

/// utility function, gets the price of a given ERC20 token in uniswap in WETH given the erc20 address and amount
/// note that this function will only query the default fee pool (0.3%)
pub async fn get_weth_price(
    token: EthAddress,
    amount: Uint256,
    pubkey: EthAddress,
    web3: &Web3,
) -> Result<Uint256, Web3Error> {
    if token == *WETH_CONTRACT_ADDRESS {
        return Ok(amount);
    } else if amount == 0u8.into() {
        return Ok(0u8.into());
    }

    let slippage_sqrt_price = web3.get_slippage_sqrt_price(token, *WETH_CONTRACT_ADDRESS, None, FIVE_PERCENT, pubkey).await?;
    let price = web3
        .get_uniswap_price(
            pubkey,
            token,
            *WETH_CONTRACT_ADDRESS,
            None,
            amount.clone(),
            Some(slippage_sqrt_price),
            None,
        )
        .await;
    price
}

/// utility function, gets the price of a given ER20 token in uniswap in DAI given the erc20 address and amount
/// note that this function will only query the default fee pool (0.3%)
pub async fn get_dai_price(
    token: EthAddress,
    amount: Uint256,
    pubkey: EthAddress,
    web3: &Web3,
) -> Result<Uint256, Web3Error> {
    if token == *DAI_CONTRACT_ADDRESS {
        return Ok(amount);
    } else if amount == 0u8.into() {
        return Ok(0u8.into());
    }

    let slippage_sqrt_price = web3.get_slippage_sqrt_price(token, *DAI_CONTRACT_ADDRESS, None, FIVE_PERCENT, pubkey).await?;
    let price = web3
        .get_uniswap_price(
            pubkey,
            token,
            *DAI_CONTRACT_ADDRESS,
            None,
            amount.clone(),
            Some(slippage_sqrt_price),
            None,
        )
        .await;
    price
}
