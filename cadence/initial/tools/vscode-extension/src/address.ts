const addressPrefix = "0x";

export function addAddressPrefix(address: string): string {
    if (address.slice(0, 2) === addressPrefix) {
        return address;
    }

    return addressPrefix + address;
}

export function removeAddressPrefix(address: string): string {
    if (address.slice(0, 2) === addressPrefix) {
        return address.slice(2);
    }

    return address;
}
