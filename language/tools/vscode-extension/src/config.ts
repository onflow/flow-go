import {commands, window, workspace} from "vscode";
import {shortAddress} from "./address";

export const ROOT_ADDR: string = shortAddress("0000000000000000000000000000000000000001");

const CONFIG_FLOW_COMMAND = "flowCommand";
const CONFIG_ROOT_ACCOUNT_KEY = "rootAccountKey";
const CONFIG_EMULATOR_ADDRESS = "emulatorAddress";
const CONFIG_NUM_ACCOUNTS = "numAccounts";

// A created account that we can submit transactions for.
type Account = {
    address: string
};

type AccountSet = {[key: string]: Account};

// The subset of extension configuration used by the language server.
type ServerConfig = {
    rootAccountKey: string
    emulatorAddress: string
};

// The config used by the extension
export class Config {
    // The name of the flow CLI executable
    flowCommand: string;
    serverConfig: ServerConfig;
    numAccounts: number;
    // Set of created accounts for which we can submit transactions.
    // Mapping from account address to account object.
    accounts: AccountSet;
    // Address of the currently active account.
    activeAccount: string;

    constructor(flowCommand: string, numAccounts: number, serverConfig: ServerConfig) {
        this.flowCommand = flowCommand;
        this.numAccounts = numAccounts;
        this.serverConfig = serverConfig;
        this.accounts = {[ROOT_ADDR]: {address: ROOT_ADDR}};
        this.activeAccount = ROOT_ADDR;
    }

    addAccount(address: string) {
        address = shortAddress(address);
        this.accounts[address] = {address: address};
    }

    // Resets account state
    resetAccounts() {
        this.accounts = {[ROOT_ADDR]: {address: ROOT_ADDR}};
        this.activeAccount = ROOT_ADDR;
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

    const rootAccountKey : string | undefined = cadenceConfig.get(CONFIG_ROOT_ACCOUNT_KEY);
    if (!rootAccountKey) {
        throw new Error(`Missing ${CONFIG_ROOT_ACCOUNT_KEY} config`);
    }

    const emulatorAddress: string | undefined = cadenceConfig.get(CONFIG_EMULATOR_ADDRESS);
    if (!emulatorAddress) {
        throw new Error(`Missing ${CONFIG_EMULATOR_ADDRESS} config`);
    }

    const numAccounts: number | undefined = cadenceConfig.get(CONFIG_NUM_ACCOUNTS);
    if (!numAccounts) {
        throw new Error(`Missing ${CONFIG_NUM_ACCOUNTS} config`);
    }

    const serverConfig = {rootAccountKey, emulatorAddress};

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

