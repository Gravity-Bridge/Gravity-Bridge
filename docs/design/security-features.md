# Security Features

The Gravity Bridge has a number of features built in to double check its own operation at runtime to ensure the security of user funds. These features notably do not put users in a vulnerable spot when significant arbirage opportunities, black swan events, or large market shifts occur on either side of the bridge.

## Cross Bridge Balance Monitoring

As part of the Orion update, end-to-end bridge balance monitoring was added. This monitoring takes a two-sided approach to add security to the bridge through changes to the Ethereum side (within the `Orchestrator`, and the `Oracle`), and the Cosmos side (within the `gravity module`).

The checks described below will effectively halt the bridge on any misbehavior until it can be remedied. They will also force any attacker to execute the entirety of the attack in one go

### Cosmos-Side Balance Monitoring

Once any Ethereum `Event` has been voted on and accepted by the `validators`, an out-of-band [Sanity Check](https://en.wikipedia.org/wiki/Sanity_check) is performed to ensure that the state transitions caused by the `Event` match up to what are acceptable. This additional check and those in *Invariants* was inspired by observing types of attacks other bridges have suffered. Often in these attacks other data becomes corrupted or unusual behavior occurs alongside the critical loss of funds, so the bridge will halt in the event of any discovered irregular behavior.

### Ethereum-Side Balance Monitoring

Within the `Orchestrator` and the `Ethereum Signer` a function call has been added to assert the historical balances of the bridge are in order before any further bridge operations are performed. Specifically the balance monitoring involves:
* Collecting a governance-controlled list of monitored ERC20 token addresses from the Cosmos-side of the bridge
* Collecting a historical snapshot of both the minted Cosmos vouchers of Ethereum-originated assets and the locked Cosmos-originated assets
* Collecting the historical balances of each monitored ERC20 token held by [Gravity.sol](/solidity/contracts/Gravity.sol) at each height where a `Gravity.sol` `event` occured.
* Comparing the balances obtained from the Cosmos-side snapshots to the `Gravity.sol` balances, if any `Gravity.sol` balance is less than the Cosmos-side balance, the `Orchestrator` and the `Ethereum Signer` will not submit anything further to the Cosmos chain.


## Invariants

Cosmos SDK based blockchains have the option to enable the [Crisis module](https://docs.cosmos.network/main/modules/crisis) and gain the benefits of running [Invariants](https://en.wikipedia.org/wiki/Invariant_(mathematics)#Invariants_in_computer_science). The gravity module has two custom invariants which give the bridge additional security.

Validators are encouraged to run invariants frequently, at least every 200 blocks. Without specifying a custom invariant check rate, validators will be assigned a rate equal to a random prime number between 15 and 200.

### Module Balance Invariant

This invariant accounts for the balance held by the gravity module by looking at each unconfirmed Batch, pooled Transaction, and pending IBC Auto Forward. If an imbalance is detected, the chain will halt promptly.

### Store Validity Invariant

Inspired by attacks which other bridges have succumbed to, this check asserts that the values stored by the gravity module are not corrupt or invalid by reading every single value from the store and calling its ValidateBasic function where applicable. If any corrupt or invalid values are detected, the bridge will halt promptly.