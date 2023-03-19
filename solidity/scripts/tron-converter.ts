import { ethers } from 'ethers';

const getEvmAddress = (base58: string) => '0x' + Buffer.from(ethers.utils.base58.decode(base58)).slice(1, -4).toString('hex');

const getBase58Address = (address: string) => {
    const evmAddress = '0x41' + address.substring(2);
    const hash = ethers.utils.sha256(ethers.utils.sha256(evmAddress));
    const checkSum = hash.substring(2, 10);
    return ethers.utils.base58.encode(evmAddress + checkSum);
};
console.log(getEvmAddress(process.env.TRON_ADDRESS || '' as string), getBase58Address(process.env.EVM_ADDRESS || '' as string));