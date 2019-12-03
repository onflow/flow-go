import {commands, ExtensionContext, window, workspace} from "vscode";
import {Extension} from "./extension";
import {startServer} from "./language-server";
import {createTerminal, resetStorage} from "./terminal";
import {ROOT_ADDR} from "./config";

// Command identifiers for locally handled commands
const RESTART_SERVER = "cadence.restartServer";
const START_EMULATOR = "cadence.runEmulator";
const STOP_EMULATOR = "cadence.stopEmulator";
const UPDATE_ACCOUNT_CODE = "cadence.updateAccountCode";
const CREATE_ACCOUNT = "cadence.createAccount";
const SWITCH_ACCOUNT = "cadence.switchActiveAccount";

// Command identifies for commands handled by the Language server
const UPDATE_ACCOUNT_CODE_SERVER = "cadence.server.updateAccountCode";
const CREATE_ACCOUNT_SERVER = "cadence.server.createAccount";

// Registers a command with VS Code so it can be invoked by the user.
function registerCommand(ctx: ExtensionContext, command: string, callback: (...args: any[]) => any) {
    ctx.subscriptions.push(commands.registerCommand(command, callback));
}

// Registers all commands that are handled by the extension (as opposed to
// those handled by the Language Server).
export function registerCommands(ext: Extension) {
    registerCommand(ext.ctx, RESTART_SERVER, restartServer(ext));
    registerCommand(ext.ctx, START_EMULATOR, startEmulator(ext));
    registerCommand(ext.ctx, STOP_EMULATOR, stopEmulator(ext));
    registerCommand(ext.ctx, UPDATE_ACCOUNT_CODE, updateAccountCode(ext));
    registerCommand(ext.ctx, CREATE_ACCOUNT, createAccount(ext));
    registerCommand(ext.ctx, SWITCH_ACCOUNT, switchActiveAccount(ext));
}

// Restarts the language server, updating the client in the extension object.
const restartServer = (ext: Extension) => async () => {
    if (!ext.client) {
        return;
    }
    await ext.client.stop();
    ext.client = startServer(ext.ctx, ext.config);
};

// Starts the emulator in a terminal window.
const startEmulator = (ext: Extension) => async () => {
    const terminal = ext.terminal;
    if (!terminal) {
        return;
    }

    // Start the emulator with the root key we gave to the language server.
    const rootKey = ext.config.serverConfig.rootAccountKey;

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
    ext.terminal = createTerminal(ext.ctx);
};

// Submits a transaction that updates the current account's code the
// code defined in the active document.
const updateAccountCode = (ext: Extension) => async () => {
    const activeEditor = window.activeTextEditor;
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

const createAccount = (ext: Extension) => async () => {
    const nextAddr = ROOT_ADDR.slice(0, -1) + (Object.keys(ext.config.accounts).length + 1)
    ext.config.accounts[nextAddr] = { address: nextAddr };
    window.showInformationMessage(`Created new account: ${nextAddr}`)
};

const switchActiveAccount = (ext: Extension) => async () => {
    // Create the options (mark the active account with an 'active' prefix)
    const accountOptions = Object
        .keys(ext.config.accounts)
        .map(addr => addr === ext.config.activeAccount ? `* ${addr}` : addr);

    window.showQuickPick(accountOptions)
        .then(selected => {
            // `selected` is undefined if the QuickPick is dismissed, and the
            // string value of the selected option otherwise.
            if (selected === undefined) {
                return;
            }
            if (!ext.config.accounts[selected]) {
                return;
            }
            ext.config.activeAccount = selected;
            window.showInformationMessage(`Switched to account ${selected}`);
        });
};
