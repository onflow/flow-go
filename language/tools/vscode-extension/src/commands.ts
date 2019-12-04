import {commands, ExtensionContext, window, workspace} from "vscode";
import {Extension} from "./extension";
import {LanguageServerAPI} from "./language-server";
import {createTerminal} from "./terminal";
import {ROOT_ADDR} from "./config";

// Command identifiers for locally handled commands
const RESTART_SERVER = "cadence.restartServer";
const START_EMULATOR = "cadence.runEmulator";
const STOP_EMULATOR = "cadence.stopEmulator";
const UPDATE_ACCOUNT_CODE = "cadence.updateAccountCode";
const CREATE_ACCOUNT = "cadence.createAccount";
const SWITCH_ACCOUNT = "cadence.switchActiveAccount";

// Command identifies for commands handled by the Language server
export const UPDATE_ACCOUNT_CODE_SERVER = "cadence.server.updateAccountCode";
export const CREATE_ACCOUNT_SERVER = "cadence.server.createAccount";
export const SWITCH_ACCOUNT_SERVER = "cadence.server.switchActiveAccount";

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
    await ext.api.client.stop();
    ext.api = new LanguageServerAPI(ext.ctx, ext.config);
};

// Starts the emulator in a terminal window.
const startEmulator = (ext: Extension) => async () => {
    // Start the emulator with the root key we gave to the language server.
    const rootKey = ext.config.serverConfig.rootAccountKey;

    ext.terminal.sendText(`${ext.config.flowCommand} emulator start --init --verbose --root-key ${rootKey}`);
    ext.terminal.show();
};

// Stops emulator, exits the terminal, and removes all config/db files.
const stopEmulator = (ext: Extension) => async () => {
    ext.terminal.dispose();
    ext.terminal = createTerminal(ext.ctx);
};

// Submits a transaction that updates the current account's code the
// code defined in the active document.
const updateAccountCode = (ext: Extension) => async () => {
    const activeEditor = window.activeTextEditor;
    if (!activeEditor) {
        return;
    }
    const activeDocumentUri = activeEditor.document.uri;

    try {
        ext.api.updateAccountCode(activeDocumentUri);
    } catch (err) {
        window.showWarningMessage("Failed to update account code");
        console.error(err);
    }
};

// Creates a new account by requesting that the Language Server submit
// a "create account" transaction from the currently active account.
const createAccount = (ext: Extension) => async () => {
    try {
        const addr = await ext.api.createAccount();
        ext.config.accounts[addr] = {address: addr};
    } catch (err) {
        window.showErrorMessage("Failed to create account: " + err);
        return;
    }
};

// Switches the active account to the option selected by the user. The selection
// is propagated to the Language Server.
const switchActiveAccount = (ext: Extension) => async () => {
    // Suffix to indicate which account is active
    const activeSuffix = "(active)";
    // Create the options (mark the active account with an 'active' prefix)
    const accountOptions = Object
        .keys(ext.config.accounts)
        // Mark the active account with a `*` in the dialog
        .map(addr => addr === ext.config.activeAccount ? `${addr} ${activeSuffix}` : addr);

    window.showQuickPick(accountOptions)
        .then(selected => {
            // `selected` is undefined if the QuickPick is dismissed, and the
            // string value of the selected option otherwise.
            if (selected === undefined) {
                return;
            }
            // If the user selected the active account, remove the `*` prefix
            if (selected.endsWith(activeSuffix)) {
                selected = selected.slice(0, -activeSuffix.length).trim();
            }
            if (!ext.config.accounts[selected]) {
                console.error('Switched to invalid account: ', selected);
                return;
            }

            try {
                ext.api.switchActiveAccount(selected);
            } catch (err) {
                window.showWarningMessage("Failed to switch active account");
                console.error(err);
                return;
            }
            ext.config.activeAccount = selected;
            window.showInformationMessage(`Switched to account ${selected}`);
        });
};
