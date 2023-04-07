

/// A mirror of the EthereumClaim interface on the Go side
/// EthereumClaim represents a claim on ethereum state
pub trait EthereumClaim {
    /// All Ethereum claims that we relay from the Gravity contract and into the module
    /// have a nonce that is strictly increasing and unique, since this nonce is
    /// issued by the Ethereum contract it is immutable and must be agreed on by all validators
    /// any disagreement on what claim goes to what nonce means someone is lying.
    fn get_event_nonce(&self) -> u64;

    /// The block height that the claimed event occurred on. This EventNonce provides sufficient
    /// ordering for the execution of all claims. The block height is used only for batchTimeouts + logicTimeouts
    /// when we go to create a new batch we set the timeout some number of batches out from the last
    /// known height plus projected block progress since then.
    fn get_eth_block_height(&self) -> u64;

    /// the delegate address of the claimer, for MsgDepositClaim and MsgWithdrawClaim
    /// this is sent in as the sdk.AccAddress of the delegated key. it is up to the user
    /// to disambiguate this into a sdk.ValAddress
    ///
    fn get_claimer(&self) -> String;

    /// Which type of claim this is
    fn get_type(&self) -> ClaimType;

    /*
    TODO: Consider implementing ClaimHash, although it should be queryable via a cosmos node
    fn validate_basic(&self) -> error;

    /// The claim hash of this claim. This is used to store these claims and also used to check if two different
    /// validators claims agree. Therefore it's extremely important that this include all elements of the claim
    /// with the exception of the orchestrator who sent it in, which will be used as a different part of the index
    fn claim_hash(&self) -> Result<Vec<u8>, error>;
     */
}

impl ToString for ClaimType {
    fn to_string(&self) -> String {
        match self {
            ClaimType::Unspecified => "CLAIM_TYPE_UNSPECIFIED".to_string(),
            ClaimType::SendToCosmos => "CLAIM_TYPE_SEND_TO_COSMOS".to_string(),
            ClaimType::BatchSendToEth => "CLAIM_TYPE_BATCH_SEND_TO_ETH".to_string(),
            ClaimType::Erc20Deployed => "CLAIM_TYPE_ERC20_DEPLOYED".to_string(),
            ClaimType::LogicCallExecuted => "CLAIM_TYPE_LOGIC_CALL_EXECUTED".to_string(),
            ClaimType::ValsetUpdated => "CLAIM_TYPE_VALSET_UPDATED".to_string(),
        }
    }
}

impl EthereumClaim for MsgSendToCosmosClaim {
    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn get_eth_block_height(&self) -> u64 {
        self.eth_block_height
    }

    fn get_claimer(&self) -> String {
        self.orchestrator.clone()
    }

    fn get_type(&self) -> ClaimType {
        ClaimType::SendToCosmos
    }
}
impl EthereumClaim for MsgBatchSendToEthClaim {
    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn get_eth_block_height(&self) -> u64 {
        self.eth_block_height
    }

    fn get_claimer(&self) -> String {
        self.orchestrator.clone()
    }

    fn get_type(&self) -> ClaimType {
        ClaimType::BatchSendToEth
    }
}
impl EthereumClaim for MsgErc20DeployedClaim {
    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn get_eth_block_height(&self) -> u64 {
        self.eth_block_height
    }

    fn get_claimer(&self) -> String {
        self.orchestrator.clone()
    }

    fn get_type(&self) -> ClaimType {
        ClaimType::Erc20Deployed
    }
}
impl EthereumClaim for MsgLogicCallExecutedClaim {
    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn get_eth_block_height(&self) -> u64 {
        self.eth_block_height
    }

    fn get_claimer(&self) -> String {
        self.orchestrator.clone()
    }

    fn get_type(&self) -> ClaimType {
        ClaimType::LogicCallExecuted
    }
}
impl EthereumClaim for MsgValsetUpdatedClaim {
    fn get_event_nonce(&self) -> u64 {
        self.event_nonce
    }

    fn get_eth_block_height(&self) -> u64 {
        self.eth_block_height
    }

    fn get_claimer(&self) -> String {
        self.orchestrator.clone()
    }

    fn get_type(&self) -> ClaimType {
        ClaimType::ValsetUpdated
    }
}
