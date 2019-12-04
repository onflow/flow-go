import {
    ExtensionContext,
    window,
    Terminal, QuickPickOptions
} from "vscode";
import {LanguageClient} from "vscode-languageclient";
import {getConfig, handleConfigChanges, Config} from "./config";
import {startServer} from "./language-server";
import {registerCommands} from "./commands";
import {createTerminal} from "./terminal";

// The container for all data relevant to the extension.
export type Extension = {
    config: Config
    ctx: ExtensionContext
    client?: LanguageClient
    terminal?: Terminal
};

// Called when the extension starts up. Reads config, starts the language
// server, and registers command handlers.
export function activate(ctx: ExtensionContext) {
    let config: Config;
    let terminal: Terminal;
    let client: LanguageClient;

    try {
        config = getConfig();
        terminal = createTerminal(ctx);
        client = startServer(ctx, config);
    } catch (err) {
        window.showErrorMessage("Failed to activate extension: ", err.msg);
        return;
    }
    handleConfigChanges();

    const ext: Extension = {
        config: config,
        ctx: ctx,
        client: client,
        terminal: terminal,
    };
    registerCommands(ext);
}

export function deactivate() {}
