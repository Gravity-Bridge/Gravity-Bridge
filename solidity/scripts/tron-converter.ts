import { ethers } from 'ethers';

const getEvmAddress = (base58: string) => '0x' + Buffer.from(ethers.utils.base58.decode(base58)).slice(1, -4).toString('hex');

const getBase58Address = (address: string) => {
    const evmAddress = '0x41' + address.substring(2);
    const hash = ethers.utils.sha256(ethers.utils.sha256(evmAddress));
    const checkSum = hash.substring(2, 10);
    return ethers.utils.base58.encode(evmAddress + checkSum);
};
console.log(getEvmAddress('TY5X9ocQACH9YGAyiK3WUxLcLw3t2ethnc'), getBase58Address('0xf2846a1E4dAFaeA38C1660a618277d67605bd2B5'));