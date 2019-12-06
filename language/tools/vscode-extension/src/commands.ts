import {commands, ExtensionContext, Position, Range, window, workspace} from "vscode";
import {Extension, renderExtension} from "./extension";
import {LanguageServerAPI} from "./language-server";
import {createTerminal} from "./terminal";
import {shortAddress, stripAddressPrefix} from "./address";

// Command identifiers for locally handled commands
export const RESTART_SERVER = "cadence.restartServer";
export const START_EMULATOR = "cadence.runEmulator";
export const STOP_EMULATOR = "cadence.stopEmulator";
export const UPDATE_ACCOUNT_CODE = "cadence.updateAccountCode";
export const CREATE_ACCOUNT = "cadence.createAccount";
export const SWITCH_ACCOUNT = "cadence.switchActiveAccount";

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

    await createDefaultAccounts(ext);
};

// Stops emulator, exits the terminal, and removes all config/db files.
const stopEmulator = (ext: Extension) => async () => {
    ext.terminal.dispose();
    ext.terminal = createTerminal(ext.ctx);

    // Clear accounts and restart language server to ensure account
    // state is in sync.
    ext.config.resetAccounts();
    await ext.api.client.stop();
    ext.api = new LanguageServerAPI(ext.ctx, ext.config);
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
        ext.config.addAccount(addr);
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
        .map(addr => addr === ext.config.activeAccount ? `${shortAddress(addr)} ${activeSuffix}` : shortAddress(addr));

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
                ext.api.switchActiveAccount(stripAddressPrefix(selected));
                window.visibleTextEditors.forEach(editor => {
                    if (!editor.document.lineCount) {
                        return;
                    }
                    // TODO We add a space to the end of the last line to force
                    // Codelens to refresh.
                    const lineCount = editor.document.lineCount;
                    const lastLine = editor.document.lineAt(lineCount-1);
                    const lastLineLen =lastLine.text.length;
                    editor.edit(edit => {
                        if (lastLine.isEmptyOrWhitespace) {
                            edit.insert(new Position(lineCount-1, 0), ' ');
                            edit.delete(new Range(lineCount-1, 0, lineCount-1, 1000));
                        } else {
                            edit.insert(new Position(lineCount-1, 1000), '\n');
                        }
                    });
                });
            } catch (err) {
                window.showWarningMessage("Failed to switch active account");
                console.error(err);
                return;
            }
            ext.config.activeAccount = selected;
            window.showInformationMessage(`Switched to account ${selected}`);
            renderExtension(ext);
        });
};

// Automatically create the number of default accounts specified in the extension configuration.
async function createDefaultAccounts(ext: Extension): Promise<void> {
    // wait 3 seconds to allow emulator to launch
    const accountCreationDelay = 3000;

    return new Promise((resolve, reject) => {
        setTimeout(async () => {
            window.showInformationMessage(`Creating ${ext.config.numAccounts} default accounts`);

            for (let i = 1; i < ext.config.numAccounts; i++) {
                try {
                    let addr = await ext.api.createAccount();
                    ext.config.addAccount(addr);
                } catch (err) {
                    window.showWarningMessage("Failed to create default account");
                    console.error(err);
                    reject(err);
                }
            }

            resolve()
        }, accountCreationDelay)
    });
}
