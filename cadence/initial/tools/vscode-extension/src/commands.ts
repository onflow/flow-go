import {commands, ExtensionContext, Position, Range, window, workspace} from "vscode";
import {Extension, renderExtension} from "./extension";
import {LanguageServerAPI} from "./language-server";
import {createTerminal} from "./terminal";
import {removeAddressPrefix} from "./address";

// Command identifiers for locally handled commands
export const RESTART_SERVER = "cadence.restartServer";
export const START_EMULATOR = "cadence.runEmulator";
export const STOP_EMULATOR = "cadence.stopEmulator";
export const CREATE_ACCOUNT = "cadence.createAccount";
export const SWITCH_ACCOUNT = "cadence.switchActiveAccount";

// Command identifies for commands handled by the Language server
export const CREATE_ACCOUNT_SERVER = "cadence.server.createAccount";
export const CREATE_DEFAULT_ACCOUNTS_SERVER = "cadence.server.createDefaultAccounts";
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
    // Start the emulator with the service key we gave to the language server.
    const {serverConfig} = ext.config

    ext.terminal.sendText(
        [
            ext.config.flowCommand,
            `emulator`, `start`, `--init`, `--verbose`,
            `--service-priv-key`, serverConfig.servicePrivateKey,
            `--service-sig-algo`, serverConfig.serviceKeySignatureAlgorithm,
            `--service-hash-algo`, serverConfig.serviceKeyHashAlgorithm,
        ].join(" ")
    );
    ext.terminal.show();

    // create default accounts after the emulator has started
    // skip service account since it is already created
    setTimeout(async () => {
        try {
            const accounts = await ext.api.createDefaultAccounts(ext.config.numAccounts - 1);
            accounts.forEach(address => ext.config.addAccount(address));
        } catch (err) {
            console.error("Failed to create default accounts", err);
            window.showWarningMessage("Failed to create default accounts");
        }
    }, 3000);
};

// Stops emulator, exits the terminal, and removes all config/db files.
const stopEmulator = (ext: Extension) => async () => {
    ext.terminal.dispose();
    ext.terminal = createTerminal(ext.ctx);

    // Clear accounts and restart language server to ensure account
    // state is in sync.
    ext.config.resetAccounts();
    renderExtension(ext);
    await ext.api.client.stop();
    ext.api = new LanguageServerAPI(ext.ctx, ext.config);
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
        .values(ext.config.accounts)
        // Mark the active account with a `*` in the dialog
        .map((account) => {
            const suffix: String = account.index === ext.config.activeAccount ? ` ${activeSuffix}` : "";
            const label = `${account.fullName()}${suffix}`;

            return {
                label: label, 
                target: account.index 
            }
        })

    window.showQuickPick(accountOptions)
        .then(selected => {
            // `selected` is undefined if the QuickPick is dismissed, and the
            // string value of the selected option otherwise.
            if (selected === undefined) {
                return;
            }

            const activeIndex = selected.target;
            const activeAccount = ext.config.getAccount(activeIndex);

            if (!activeAccount) {
                console.error('Switched to invalid account');
                return;
            }

            try {
                ext.api.switchActiveAccount(removeAddressPrefix(activeAccount.address));
                window.visibleTextEditors.forEach(editor => {
                    if (!editor.document.lineCount) {
                        return;
                    }
                    // NOTE: We add a space to the end of the last line to force
                    // Codelens to refresh.
                    const lineCount = editor.document.lineCount;
                    const lastLine = editor.document.lineAt(lineCount-1);
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

            ext.config.setActiveAccount(activeIndex)

            window.showInformationMessage(`Switched to account ${activeAccount.fullName()}`);

            renderExtension(ext);
        });
};
