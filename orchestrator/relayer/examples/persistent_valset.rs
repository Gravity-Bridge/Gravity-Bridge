use clarity::Uint256;
use gravity_utils::types::{Valset, ValsetMember};

fn main() {
    let value: (Uint256, Valset) = (
        100u128.into(),
        Valset {
            nonce: 1,
            members: vec![ValsetMember {
                power: 10,
                eth_address: "0xc9B6f87d637d4774EEB54f8aC2b89dBC3D38226b"
                    .parse()
                    .unwrap(),
            }],
            reward_amount: 10u128.into(),
            reward_token: None,
        },
    );

    std::fs::write(
        format!("/tmp/relayer/{}", "oraib.json"),
        serde_json::to_string(&value).unwrap(),
    )
    .unwrap();
}
