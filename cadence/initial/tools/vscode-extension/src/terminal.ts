import {ExtensionContext, Terminal, window} from "vscode";
import {existsSync, mkdirSync, unlinkSync} from "fs";
import {join} from "path";

// Name of all Flow files stored on-disk.
const FLOW_CONFIG_FILENAME = "flow.json";
const FLOW_DB_FILENAME = "flowdb";

// Creates a terminal within VS Code.
export function createTerminal(ctx: ExtensionContext): Terminal {
    const storagePath = getStoragePath(ctx);
    if (!storagePath) {
        throw new Error("Missing extension storage path");
    }

    // By default, reset all files on each load.
    resetStorage(ctx);

    return window.createTerminal({
        name: "Flow Emulator",
        hideFromUser: true,
        cwd: storagePath,
    });
}

// Deletes all Flow files from extension storage.
// TODO: This doesn't work right now due to permissions issue
// REF: https://github.com/dapperlabs/flow-go/issues/1726
export function resetStorage(ctx: ExtensionContext) {
    const storagePath = ctx.storagePath;
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
function getStoragePath(ctx: ExtensionContext): string | undefined {
     let storagePath = ctx.storagePath;
    if (!storagePath) {
        storagePath = ctx.globalStoragePath;
    }
    console.log("Storage path: ", storagePath);
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
