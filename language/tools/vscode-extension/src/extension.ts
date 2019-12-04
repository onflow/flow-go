import {
    ExtensionContext,
    window,
    Terminal,
} from "vscode";
import {getConfig, handleConfigChanges, Config} from "./config";
import {LanguageServerAPI} from "./language-server";
import {registerCommands} from "./commands";
import {createTerminal} from "./terminal";

// The container for all data relevant to the extension.
export type Extension = {
    config: Config
    ctx: ExtensionContext
    api: LanguageServerAPI
    terminal: Terminal
};

// Called when the extension starts up. Reads config, starts the language
// server, and registers command handlers.
export function activate(ctx: ExtensionContext) {
    let config: Config;
    let terminal: Terminal;
    let api: LanguageServerAPI;

    try {
        config = getConfig();
        terminal = createTerminal(ctx);
        api = new LanguageServerAPI(ctx, config);
    } catch (err) {
        window.showErrorMessage("Failed to activate extension: ", err.msg);
        return;
    }
    handleConfigChanges();

    const ext: Extension = {
        config: config,
        ctx: ctx,
        api: api,
        terminal: terminal,
    };
    registerCommands(ext);
}

export function deactivate() {}
