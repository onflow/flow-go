import {commands, window, workspace} from "vscode";
import { addAddressPrefix } from "./address";

export const SERVICE_ADDR: string = "f8d6e0586b0a20c7";

const CONFIG_FLOW_COMMAND = "flowCommand";
const CONFIG_SERVICE_PRIVATE_KEY = "servicePrivateKey";
const CONFIG_SERVICE_KEY_SIGNATURE_ALGORITHM = "serviceKeySignatureAlgorithm";
const CONFIG_SERVICE_KEY_HASH_ALGORITHM = "serviceKeyHashAlgorithm";
const CONFIG_EMULATOR_ADDRESS = "emulatorAddress";
const CONFIG_NUM_ACCOUNTS = "numAccounts";

// An account that can be used to submit transactions.
export class Account {
    index: number
    address: string

    constructor(index: number, address: string) {
        this.index = index;
        this.address = address;
    }

    name(): string {
        return this.index === 0 ? "Service Account" : `Account ${this.index}`;
    }

    fullName(): string {
        return `${this.name()} (${addAddressPrefix(this.address)})`;
    }
};

// The subset of extension configuration used by the language server.
type ServerConfig = {
    servicePrivateKey: string
    serviceKeySignatureAlgorithm: string
    serviceKeyHashAlgorithm: string
    emulatorAddress: string
};

// The configuration used by the extension.
export class Config {
    // The name of the flow CLI executable
    flowCommand: string;
    serverConfig: ServerConfig;
    numAccounts: number;
    // Set of created accounts for which we can submit transactions.
    // Mapping from account address to account object.
    accounts: Array<Account>;
    // Index of the currently active account.
    activeAccount: number;

    constructor(flowCommand: string, numAccounts: number, serverConfig: ServerConfig) {
        this.flowCommand = flowCommand;
        this.numAccounts = numAccounts;
        this.serverConfig = serverConfig;
        this.accounts = [new Account(0, SERVICE_ADDR)];
        this.activeAccount = 0;
    }

    addAccount(address: string) {
        const index = this.accounts.length;
        this.accounts.push(new Account(index, address));
    }

    setActiveAccount(index: number) {
        this.activeAccount = index;
    }
    
    getActiveAccount(): Account {
        return this.accounts[this.activeAccount];
    }

    getAccount(index: number): Account|null {
        if (index < 0 || index >= this.accounts.length) {
            return null;
        }

        return this.accounts[index]
    }

    // Resets account state
    resetAccounts() {
        this.accounts = [new Account(0, SERVICE_ADDR)];
        this.activeAccount = 0;
    }
}

// Retrieves config from the workspace.
export function getConfig(): Config {
    const cadenceConfig = workspace
        .getConfiguration("cadence");

    const flowCommand: string | undefined = cadenceConfig.get(CONFIG_FLOW_COMMAND);
    if (!flowCommand) {
        throw new Error(`Missing ${CONFIG_FLOW_COMMAND} config`);
    }

    const servicePrivateKey: string | undefined = cadenceConfig.get(CONFIG_SERVICE_PRIVATE_KEY);
    if (!servicePrivateKey) {
        throw new Error(`Missing ${CONFIG_SERVICE_PRIVATE_KEY} config`);
    }

    const serviceKeySignatureAlgorithm: string | undefined = cadenceConfig.get(CONFIG_SERVICE_KEY_SIGNATURE_ALGORITHM);
    if (!serviceKeySignatureAlgorithm) {
        throw new Error(`Missing ${CONFIG_SERVICE_KEY_SIGNATURE_ALGORITHM} config`);
    }

    const serviceKeyHashAlgorithm: string | undefined = cadenceConfig.get(CONFIG_SERVICE_KEY_HASH_ALGORITHM);
    if (!serviceKeyHashAlgorithm) {
        throw new Error(`Missing ${CONFIG_SERVICE_KEY_HASH_ALGORITHM} config`);
    }

    const emulatorAddress: string | undefined = cadenceConfig.get(CONFIG_EMULATOR_ADDRESS);
    if (!emulatorAddress) {
        throw new Error(`Missing ${CONFIG_EMULATOR_ADDRESS} config`);
    }

    const numAccounts: number | undefined = cadenceConfig.get(CONFIG_NUM_ACCOUNTS);
    if (!numAccounts) {
        throw new Error(`Missing ${CONFIG_NUM_ACCOUNTS} config`);
    }

    const serverConfig: ServerConfig = {
        servicePrivateKey,
        serviceKeySignatureAlgorithm,
        serviceKeyHashAlgorithm,
        emulatorAddress
    };

    return new Config(flowCommand, numAccounts, serverConfig);
}

// Adds an event handler that prompts the user to reload whenever the config
// changes.
export function handleConfigChanges() {
    workspace.onDidChangeConfiguration(e => {
        // TODO: do something smarter for account/emulator config (re-send to server)
        const promptRestartKeys = ["languageServerPath", "accountKey", "accountAddress", "emulatorAddress"];
        const shouldPromptRestart = promptRestartKeys.some(key =>
            e.affectsConfiguration(`cadence.${key}`)
        );
        if (shouldPromptRestart) {
            window
                .showInformationMessage(
                    "Server launch configuration change detected. Reload the window for changes to take effect",
                    "Reload Window",
                    "Not now"
                )
                .then(choice => {
                    if (choice === "Reload Window") {
                        commands.executeCommand("workbench.action.reloadWindow");
                    }
                });
        }
    });
}

