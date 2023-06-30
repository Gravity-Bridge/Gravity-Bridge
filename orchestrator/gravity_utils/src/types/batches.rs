use super::*;
use crate::error::GravityError;
use crate::num_conversion::print_eth;
use crate::prices::{get_dai_price, get_weth_price};
use clarity::Signature as EthSignature;
use clarity::{abi::AbiToken as Token, Address as EthAddress};
use deep_space::Address as CosmosAddress;
use log::LevelFilter;
use std::convert::TryFrom;
use tokio::join;
use web30::client::Web3;

/// This represents an individual transaction being bridged over to Ethereum
/// parallel is the OutgoingTransferTx in x/gravity/types/batch.go
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct BatchTransaction {
    /// This transactions Cosmos pool id
    pub id: u64,
    /// The senders Cosmos address
    pub sender: CosmosAddress,
    pub destination: EthAddress,
    /// The fee that is being paid, must be of the same
    /// ERC20 type as erc20_fee
    pub erc20_token: Erc20Token,
    /// The fee that is being paid, must be of the same
    /// ERC20 type as erc20_token
    pub erc20_fee: Erc20Token,
}

/// The parsed version of a transaction batch, representing
/// a set of tokens that may be brought over the bridge to Ethereum
/// as a single operation paid for by a relayer
#[derive(Serialize, Deserialize, Debug, Default, Clone)]
pub struct TransactionBatch {
    /// This batches nonce
    pub nonce: u64,
    /// this batches timeout value in terms of Ethereum block height
    pub batch_timeout: u64,
    /// transactions contained in this batch
    pub transactions: Vec<BatchTransaction>,
    /// the total of the erc20_fee values in transactions
    pub total_fee: Erc20Token,
    /// the ERC20 token contract shared by all transactions
    /// and fees in this batch
    pub token_contract: EthAddress,
}

impl TransactionBatch {
    /// extracts the amounts, destinations and fees as submitted to the Ethereum contract
    /// and used for signatures
    pub fn get_checkpoint_values(&self) -> (Token, Token, Token) {
        let mut amounts = Vec::new();
        let mut destinations = Vec::new();
        let mut fees = Vec::new();
        for item in self.transactions.iter() {
            amounts.push(Token::Uint(item.erc20_token.amount));
            fees.push(Token::Uint(item.erc20_fee.amount));
            destinations.push(item.destination)
        }
        assert_eq!(amounts.len(), destinations.len());
        assert_eq!(fees.len(), destinations.len());
        (
            Token::Dynamic(amounts),
            destinations.into(),
            Token::Dynamic(fees),
        )
    }

    /// this function displays info about this batch including metadata
    /// such as the name of the ERC20, it's current value etc
    pub async fn display_with_eth_info(&self, pubkey: EthAddress, web30: &Web3) {
        let level = log::max_level();
        // do not run all these queries if logging is set below info
        if LevelFilter::Info > level {
            return;
        }

        let token = self.token_contract;
        let fee_total = self.total_fee.amount;
        let mut tx_total: Uint256 = 0u8.into();
        for tx in self.transactions.clone() {
            tx_total += tx.erc20_token.amount;
        }
        let fee_value_weth = get_weth_price(pubkey, token, fee_total, web30);
        let fee_value_dai = get_dai_price(pubkey, token, fee_total, web30);
        let tx_value_weth = get_weth_price(pubkey, token, tx_total, web30);
        let tx_value_dai = get_dai_price(pubkey, token, tx_total, web30);
        let token_symbol = web30.get_erc20_symbol(token, pubkey);
        let current_block = web30.eth_block_number();
        if let (
            Ok(fee_value_weth),
            Ok(fee_value_dai),
            Ok(tx_value_weth),
            Ok(tx_value_dai),
            Ok(token_symbol),
            Ok(current_block),
        ) = join!(
            fee_value_weth,
            fee_value_dai,
            tx_value_weth,
            tx_value_dai,
            token_symbol,
            current_block,
        ) {
            info!("Batch Info:");
            info!("Token: {}  Contract Address: {}", token_symbol, token);
            info!(
                "Contains {} transactions, total value {} DAI {} ETH",
                self.transactions.len(),
                print_eth(tx_value_dai),
                print_eth(tx_value_weth)
            );
            info!(
                "Total fee value {} DAI, {} ETH",
                print_eth(fee_value_dai),
                print_eth(fee_value_weth)
            );
            if current_block < self.batch_timeout.into() {
                let batch_timeout: Uint256 = self.batch_timeout.into();
                info!("Timeout in {} blocks", batch_timeout - current_block)
            } else {
                info!("Batch is timed out and can't be submitted")
            }
        } else {
            info!("Batch Info:");
            info!("Contract Address: {}", token);
            info!(
                "Contains {} transactions, total value {}",
                self.transactions.len(),
                tx_total,
            );
            info!("Total fee value {}", fee_total);
            info!("Timeout block is {}", self.batch_timeout);
        }
    }
}

/// the response we get when querying for a batch confirmation
#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct BatchConfirmResponse {
    pub nonce: u64,
    pub orchestrator: CosmosAddress,
    pub token_contract: EthAddress,
    pub ethereum_signer: EthAddress,
    pub eth_signature: EthSignature,
}

impl Confirm for BatchConfirmResponse {
    fn get_eth_address(&self) -> EthAddress {
        self.ethereum_signer
    }
    fn get_signature(&self) -> EthSignature {
        self.eth_signature.clone()
    }
}

impl TryFrom<gravity_proto::gravity::OutgoingTxBatch> for TransactionBatch {
    type Error = GravityError;
    fn try_from(
        input: gravity_proto::gravity::OutgoingTxBatch,
    ) -> Result<TransactionBatch, GravityError> {
        let mut transactions = Vec::new();
        let mut running_total_fee: Option<Erc20Token> = None;
        for tx in input.transactions {
            let tx = BatchTransaction::try_from(tx)?;
            if let Some(total_fee) = running_total_fee {
                running_total_fee = Some(Erc20Token {
                    token_contract_address: total_fee.token_contract_address,
                    amount: total_fee.amount + tx.erc20_fee.amount,
                });
            } else {
                running_total_fee = Some(tx.erc20_fee.clone())
            }
            transactions.push(tx);
        }
        if let Some(total_fee) = running_total_fee {
            Ok(TransactionBatch {
                batch_timeout: input.batch_timeout,
                nonce: input.batch_nonce,
                transactions,
                token_contract: total_fee.token_contract_address,
                total_fee,
            })
        } else {
            Err(GravityError::InvalidBridgeStateError(
                "Transaction batch containing no transactions!".to_string(),
            ))
        }
    }
}

impl TryFrom<gravity_proto::gravity::MsgConfirmBatch> for BatchConfirmResponse {
    type Error = GravityError;
    fn try_from(
        input: gravity_proto::gravity::MsgConfirmBatch,
    ) -> Result<BatchConfirmResponse, GravityError> {
        Ok(BatchConfirmResponse {
            nonce: input.nonce,
            orchestrator: input.orchestrator.parse()?,
            token_contract: input.token_contract.parse()?,
            ethereum_signer: input.eth_signer.parse()?,
            eth_signature: input.signature.parse()?,
        })
    }
}

impl TryFrom<gravity_proto::gravity::OutgoingTransferTx> for BatchTransaction {
    type Error = GravityError;
    fn try_from(
        input: gravity_proto::gravity::OutgoingTransferTx,
    ) -> Result<BatchTransaction, GravityError> {
        if input.erc20_fee.is_none()
            || input.erc20_token.is_none()
            || input.erc20_fee.clone().unwrap().contract
                != input.erc20_token.clone().unwrap().contract
        {
            return Err(GravityError::InvalidBridgeStateError(
                "Can not have tx with null erc20_token!".to_string(),
            ));
        }
        Ok(BatchTransaction {
            id: input.id,
            sender: input.sender.parse()?,
            destination: input.dest_address.parse()?,
            erc20_token: Erc20Token::try_from(input.erc20_token.unwrap())?,
            erc20_fee: Erc20Token::try_from(input.erc20_fee.unwrap())?,
        })
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::OutgoingTransferTx> for &BatchTransaction {
    fn into(self) -> gravity_proto::gravity::OutgoingTransferTx {
        gravity_proto::gravity::OutgoingTransferTx {
            id: self.id,
            sender: self.sender.to_string(),
            dest_address: self.destination.to_string(),
            erc20_token: Some(self.erc20_token.clone().into()),
            erc20_fee: Some(self.erc20_fee.clone().into()),
        }
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::OutgoingTransferTx> for BatchTransaction {
    fn into(self) -> gravity_proto::gravity::OutgoingTransferTx {
        let r = &self;
        r.into()
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::OutgoingTxBatch> for &TransactionBatch {
    fn into(self) -> gravity_proto::gravity::OutgoingTxBatch {
        gravity_proto::gravity::OutgoingTxBatch {
            batch_nonce: self.nonce,
            batch_timeout: self.batch_timeout,
            transactions: self.transactions.iter().map(|v| v.into()).collect(),
            token_contract: self.token_contract.to_string(),
            cosmos_block_created: 0,
        }
    }
}

#[allow(clippy::from_over_into)]
impl Into<gravity_proto::gravity::OutgoingTxBatch> for TransactionBatch {
    fn into(self) -> gravity_proto::gravity::OutgoingTxBatch {
        let r = &self;
        r.into()
    }
}
