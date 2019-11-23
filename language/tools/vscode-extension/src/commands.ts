import {commands, ExtensionContext, window, workspace} from "vscode";
import {Extension} from "./extension";
import {startServer} from "./language-server";

// Command identifiers
const RESTART_SERVER = "cadence.restartServer";
const START_EMULATOR = "cadence.startEmulator";

// Registers a command with VS Code so it can be invoked by the user.
function registerCommand(ctx: ExtensionContext, command: string, callback: (...args: any[]) => any) {
    ctx.subscriptions.push(commands.registerCommand(command, callback));
}

export function registerCommands(ext: Extension) {
    registerCommand(ext.ctx, RESTART_SERVER, restartServer(ext));
    registerCommand(ext.ctx, START_EMULATOR, startEmulator(ext));
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

    terminal.sendText(`${ext.config.flowCommand} init`);
    terminal.sendText(`${ext.config.flowCommand} emulator start`);
    terminal.show();
};
