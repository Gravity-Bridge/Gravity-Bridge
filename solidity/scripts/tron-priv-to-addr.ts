// @ts-ignore
import TronWeb from 'tronweb';

const privKeyToAddr = () => {
    const tronWeb = new TronWeb({
        fullHost: process.env.TRON_HOST || 'https://nile.trongrid.io',
        headers: process.env.TRON_HEADERS,
        privateKey: process.env.PRIVATE_KEY // without '0x'
    })
    console.log("default address: ", tronWeb.defaultAddress)
}

privKeyToAddr();