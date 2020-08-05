import {
    ExtensionContext,
    window,
    Terminal,
    StatusBarItem,
} from "vscode";
import {getConfig, handleConfigChanges, Config} from "./config";
import {LanguageServerAPI} from "./language-server";
import {registerCommands} from "./commands";
import {createTerminal} from "./terminal";
import {createActiveAccountStatusBarItem, updateActiveAccountStatusBarItem} from "./status-bar";

// The container for all data relevant to the extension.
export type Extension = {
    config: Config
    ctx: ExtensionContext
    api: LanguageServerAPI
    terminal: Terminal
    activeAccountStatusBarItem: StatusBarItem
};

// Called when the extension starts up. Reads config, starts the language
// server, and registers command handlers.
export function activate(ctx: ExtensionContext) {
    let config: Config;
    let terminal: Terminal;
    let activeAccountStatusBarItem: StatusBarItem;
    let api: LanguageServerAPI;

    try {
        config = getConfig();
        terminal = createTerminal(ctx);
        api = new LanguageServerAPI(ctx, config);
        activeAccountStatusBarItem = createActiveAccountStatusBarItem();
    } catch (err) {
        window.showErrorMessage("Failed to activate extension: ", err);
        return;
    }
    handleConfigChanges();

    const ext: Extension = {
        config: config,
        ctx: ctx,
        api: api,
        terminal: terminal,
        activeAccountStatusBarItem: activeAccountStatusBarItem,
    };

    registerCommands(ext);
    renderExtension(ext);
}

export function deactivate() {}

export function renderExtension(ext: Extension) {
    updateActiveAccountStatusBarItem(ext.activeAccountStatusBarItem, ext.config.getActiveAccount());
}
