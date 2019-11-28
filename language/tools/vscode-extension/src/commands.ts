import {commands, ExtensionContext, window, workspace} from "vscode";
import {Extension} from "./extension";
import {startServer} from "./language-server";
import {createTerminal, resetStorage} from "./terminal";

// Command identifiers
const RESTART_SERVER = "cadence.restartServer";
const START_EMULATOR = "cadence.runEmulator";
const STOP_EMULATOR = "cadence.stopEmulator";
const UPDATE_ACCOUNT_CODE = "cadence.updateAccountCode";
const UPDATE_ACCOUNT_CODE_SERVER = "cadence.server.updateAccountCode";

// Registers a command with VS Code so it can be invoked by the user.
function registerCommand(ctx: ExtensionContext, command: string, callback: (...args: any[]) => any) {
    ctx.subscriptions.push(commands.registerCommand(command, callback));
}

export function registerCommands(ext: Extension) {
    registerCommand(ext.ctx, RESTART_SERVER, restartServer(ext));
    registerCommand(ext.ctx, START_EMULATOR, startEmulator(ext));
    registerCommand(ext.ctx, STOP_EMULATOR, stopEmulator(ext));
    registerCommand(ext.ctx, UPDATE_ACCOUNT_CODE, updateAccountCode(ext));
}

// Restarts the language server, updating the client in the extension object.
const restartServer = (ext: Extension) => async () => {
    if (!ext.client) {
        return;
    }
    await ext.client.stop();
    ext.client = startServer(ext);
};

// Starts the emulator in a terminal window.
const startEmulator = (ext: Extension) => async () => {
    const terminal = ext.terminal;
    if (!terminal) {
        return;
    }

    // Start the emulator with the root key we gave to the language server.
    const rootKey = ext.config.serverConfig.accountKey;

    terminal.sendText(`${ext.config.flowCommand} emulator start --init --verbose --root-key ${rootKey}`);
    terminal.show();
};

// Stops emulator, exits the terminal, and removes all config/db files.
const stopEmulator = (ext: Extension) => async () => {
    let terminal = ext.terminal;
    if (!terminal) {
        return;
    }

    terminal.dispose();
    ext.terminal = createTerminal(ext);
};

// Submits a transaction that updates the configured account's code the
// code defined in the active document.
const updateAccountCode = (ext: Extension) => async () => {
    const activeEditor = window.activeTextEditor
    if (!activeEditor) {
        return;
    }
    if (!ext.client) {
        return;
    }

    try {
        ext.client.sendRequest("workspace/executeCommand", {
            command: UPDATE_ACCOUNT_CODE_SERVER,
            arguments: [activeEditor.document.uri.toString()],
        });
    } catch (err) {
        window.showWarningMessage("Failed to update account code");
        console.error(err);
    }
};
