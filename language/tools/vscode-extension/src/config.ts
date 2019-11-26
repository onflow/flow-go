import {commands, window, workspace} from "vscode";

// The config used by the extension
export type Config = {
    flowCommand: string
    serverConfig: ServerConfig
};

// The subset of extension configuration used by the language server.
type ServerConfig = {
    accountKey: string
    accountAddress: string
    emulatorAddress: string
};

// Retrieves config from the workspace.
export function getConfig(): Config | undefined {
    const cadenceConfig = workspace
        .getConfiguration("cadence");

    const flowCommand: string | undefined = cadenceConfig.get("flowCommand")
    if (!flowCommand) {
        return;
    }

    const accountKey : string | undefined = cadenceConfig.get("accountKey");
    if (!accountKey) {
        return;
    }

    const accountAddress: string | undefined = cadenceConfig.get("accountAddress");
    if (!accountAddress) {
        return;
    }

    const emulatorAddress: string | undefined = cadenceConfig.get("emulatorAddress");
    if (!emulatorAddress) {
        return;
    }

    return {
        flowCommand: flowCommand,
        serverConfig: {
            accountKey: accountKey,
            accountAddress: accountAddress,
            emulatorAddress: emulatorAddress,
        },
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

