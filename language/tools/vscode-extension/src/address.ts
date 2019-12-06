// Format an address by trimming leading zeros and enforcing an even length.
export function formatAddress(address: string): string {
    const addressInt = parseInt(address, 16)

    let addressStr = addressInt.toString(16);
    if (addressStr.length % 2 != 0) {
        addressStr = `0${addressStr}`
    }

    return `0x${addressStr}`
}
