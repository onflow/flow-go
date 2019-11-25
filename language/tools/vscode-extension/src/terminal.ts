import {Terminal, window} from "vscode";
import {existsSync, mkdirSync, unlinkSync} from "fs";
import {Extension} from "./extension";
import {join} from "path";

// Name of all Flow files stored on-disk.
const FLOW_CONFIG_FILENAME = "flow.json";
const FLOW_DB_FILENAME = "flowdb";

// Creates a terminal within VS Code.
export function createTerminal(ext: Extension): Terminal | undefined {
    const storagePath = getStoragePath(ext);
    if (!storagePath) {
        window.showWarningMessage("Failed to start emulator: missing extension storage");
        return;
    }

    // By default, reset all files on each load.
    resetStorage(ext);

    return window.createTerminal({
        name: "Flow Emulator",
        hideFromUser: true,
        cwd: storagePath,
    });
}

// Deletes all Flow files from extension storage.
export function resetStorage(ext: Extension) {
    const storagePath = ext.ctx.storagePath;
    if (!storagePath) {
        return;
    }

    try {
        unlinkSync(join(storagePath, FLOW_CONFIG_FILENAME));
        unlinkSync(join(storagePath, FLOW_DB_FILENAME));
    } catch (err) {
        if (err.code === 'ENOENT') {
            return;
        }
        console.error("Error resetting storage: ", err);
    }
}

// Returns a path to a directory that can be used for persistent storage.
// Creates the directory if it doesn't already exist.
function getStoragePath(ext: Extension): string | undefined {
    const storagePath = ext.ctx.storagePath;
    if (!storagePath) {
        return;
    }
    if (!existsSync(storagePath)) {
        try {
            mkdirSync(storagePath);
        } catch (err) {
            console.log("Error creating storage path: ", err);
            return;
        }
    }
    return storagePath;
}
