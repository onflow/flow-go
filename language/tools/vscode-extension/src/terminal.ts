import {Terminal, window} from "vscode";
import {existsSync, mkdirSync} from "fs";
import {Extension} from "./extension";

// Creates a terminal within VS Code.
export function createTerminal(ext: Extension): Terminal | undefined {
    const storagePath = ext.ctx.storagePath;
    if (!storagePath) {
        window.showWarningMessage("Failed to start emulator: missing extension storage");
        return;
    }
    if (!existsSync(storagePath)) {
        try {
            mkdirSync(storagePath);
        } catch (err) {
            window.showWarningMessage("Failed to start emulator: unable to create config file");
            console.log(err);
            return;
        }
    }

    return window.createTerminal({
        name: "Flow Emulator",
        hideFromUser: true,
        cwd: storagePath,
    });
}
