import {window, workspace} from "vscode";

// The config used by the extension
export type Config = {
    languageServerCommand: string
    languageServerArgs: string[]
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

    const languageServerCommandRaw: string | undefined = cadenceConfig.get("languageServerCommand")
    if (!languageServerCommandRaw) {
        return;
    }
    const commandAndArgs = languageServerCommandRaw.split(/\s+/);
    if (commandAndArgs.length < 1) {
        window.showWarningMessage("Malformed language server command");
        return;
    }
    const command = commandAndArgs[0];
    const args = commandAndArgs.splice(1);

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
        languageServerCommand: command,
        languageServerArgs: args,
        serverConfig: {
            accountKey: accountKey,
            accountAddress: accountAddress,
            emulatorAddress: emulatorAddress,
        },
    };
}

