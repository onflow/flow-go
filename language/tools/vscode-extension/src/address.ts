const addressPrefix = "0x";

// Format an address by trimming leading zeros and enforcing an even length.
export function shortAddress(address: string): string {
    const addressInt = parseInt(address, 16);

    let addressStr = addressInt.toString(16);
    if (addressStr.length % 2 !== 0) {
        addressStr = `0${addressStr}`;
    }

    return `${addressPrefix}${addressStr}`;
}

export function stripAddressPrefix(address: string): string {
    if (address.slice(0, 2) === addressPrefix) {
        return address.slice(2);
    }

    return address;
}
