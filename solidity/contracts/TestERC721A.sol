//SPDX-License-Identifier: Apache-2.0
pragma solidity 0.8.10;
import "@openzeppelin/contracts/token/ERC721/ERC721.sol";

// Generate NFTs with token ids 1-10 and 190-195
contract TestERC721A is ERC721 {
	constructor() ERC721("NFT PUNK", "NFTPUNK") {
	  uint i=0;
	  // mint group 1 of nfts starting at token id 1, to 10
      for (i = 1; i <= 10; i += 1) { 
         _mint(0xc783df8a850f42e7F7e57013759C285caa701eB6, i);
      }
	  // mint group 2 of nfts starting at token id 190, to 199
	  for (i = 190; i < 200; i += 1) { 
         _mint(0xc783df8a850f42e7F7e57013759C285caa701eB6, i);
	  }
	  // mint group 3 of nfts token id 200, 201, 202
	  _mint(0xBf660843528035a5A4921534E156a27e64B231fE, 200);
	  _mint(0xBf660843528035a5A4921534E156a27e64B231fE, 201);
	  _mint(0xBf660843528035a5A4921534E156a27e64B231fE, 202);
	  _mint(0xBf660843528035a5A4921534E156a27e64B231fE, 203);
	}
}
