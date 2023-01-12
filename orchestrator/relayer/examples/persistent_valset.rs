use clarity::Uint256;
use gravity_utils::types::{Valset, ValsetMember};

fn main() {
    let value: (Uint256, Option<Valset>) = (
        100u128.into(),
        Some(Valset {
            nonce: 1,
            members: vec![ValsetMember {
                power: 10,
                eth_address: "0xc9B6f87d637d4774EEB54f8aC2b89dBC3D38226b"
                    .parse()
                    .unwrap(),
            }],
            reward_amount: 10u128.into(),
            reward_token: None,
        }),
    );

    let file_path = format!("/tmp/relayer/{}", "oraib.json");
    std::fs::write(file_path.clone(), serde_json::to_vec(&value).unwrap()).unwrap();

    let value: (Uint256, Option<Valset>) =
        serde_json::from_slice(std::fs::read(file_path).unwrap().as_slice())
            .unwrap_or((0u8.into(), None));

    println!("value {:?}", value);
}
