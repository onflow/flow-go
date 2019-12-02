import {
    ExtensionContext,
    window,
    Terminal
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
    const maybeConfig: Config | undefined = getConfig();
    if (!maybeConfig) {
        window.showWarningMessage("Missing required config");
    }
    const config = maybeConfig as Config;
    handleConfigChanges();

    const ext: Extension = {
        config: config,
        ctx: ctx,
    };

    ext.client = startServer(ext);
    ext.terminal = createTerminal(ext);


    registerCommands(ext);
}

export function deactivate() {}
