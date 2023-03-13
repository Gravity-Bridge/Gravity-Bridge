import { ethers } from 'ethers';

const getEvmAddress = (base58: string) => '0x' + Buffer.from(ethers.utils.base58.decode(base58)).slice(1, -4).toString('hex');

const getBase58Address = (address: string) => {
    const evmAddress = '0x41' + address.substring(2);
    const hash = ethers.utils.sha256(ethers.utils.sha256(evmAddress));
    const checkSum = hash.substring(2, 10);
    return ethers.utils.base58.encode(evmAddress + checkSum);
};
console.log(getEvmAddress('TVf8hwYiMa91Pd5y424xz4ybMhbqDTVN4B'), getBase58Address('0xD7F771664541b3f647CBA2be9Ab1Bc121bEEC913'));