# Apollo UPGRADE

> Apollo is the classical Greek and Roman God of Oracles, among other things. We believe that on-chain token exchange capabilities like the Auction module provides are the future, and we hope Apollo agrees.

The *Apollo* upgrade contains the following changes.

## Summary of Changes

* Add a new Param to the Gravity module: ChainFeeAuctionPoolFraction
    * This Param controls the fraction of the Send to Eth ChainFee that will be sent to the Auction Pool for use in future auctions.
* Add the Auction Module
    * The Auction module is an entirely separate module from Gravity and allows users to buy balances locked in the Auction Pool with their GRAV. The Auction module is controlled by a number of important Params which dictate the auction/bid flow. At the execution of the Apollo upgrade, the Auction module will transfer all the balances in the Auction Pool (except for GRAV) into the Auction module account. These balances will be up for Auction for roughly 7 days, during which anyone may bid on an auction by submitting a MsgBid. Each Bid requires paying at least 3110 GRAV as a fee (which will be distributed to stakers) and locks the Bid amount provided (also GRAV) in the Auction module, if the highest bidder is ever outbid, then the previous bidder will have their locked GRAV returned. At the end of the 7 day auction period the highest bidder for each auction will be given the full balance of the auction tokens, and their Bid GRAV will burned. Additionally at the end of the auction period, all new Auction Pool balances will be put up for Auction, and the cycle will continue. If an auction receives no valid bids then the auction's locked balance will be recycled into the Auction Pool for future auctions. The Param values controlling the auction modules were determined with Proposals #200-#203 and can be adjusted at any time via governance proposals.
