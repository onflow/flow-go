import {commands, window, workspace} from "vscode";

export const ROOT_ADDR: string = "0000000000000000000000000000000000000001";

const CONFIG_FLOW_COMMAND = "flowCommand";
const CONFIG_ROOT_ACCOUNT_KEY = "rootAccountKey";
const CONFIG_EMULATOR_ADDRESS = "emulatorAddress";

// A created account that we can submit transactions for.
type Account = {
    address: string
};

// The config used by the extension
export type Config = {
    // The name of the flow CLI executable
    flowCommand: string
    serverConfig: ServerConfig
    // Set of created accounts for which we can submit transactions.
    // Mapping from account address to account object.
    accounts: {[key: string]: Account},
    // Address of the current
    activeAccount: string
};

// The subset of extension configuration used by the language server.
type ServerConfig = {
    rootAccountKey: string
    emulatorAddress: string
};

// Retrieves config from the workspace.
export function getConfig(): Config {
    const cadenceConfig = workspace
        .getConfiguration("cadence");

    const flowCommand: string | undefined = cadenceConfig.get(CONFIG_FLOW_COMMAND)
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

    return {
        flowCommand: flowCommand,
        serverConfig: {
            rootAccountKey: rootAccountKey,
            emulatorAddress: emulatorAddress,
        },
        accounts: {
            [ROOT_ADDR]: {address: ROOT_ADDR}
        },
        activeAccount: ROOT_ADDR,
    };
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

