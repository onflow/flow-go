"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : new P(function (resolve) { resolve(result.value); }).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
const vscode_1 = require("vscode");
const vscode_languageclient_1 = require("vscode-languageclient");
function activate(ctx) {
    detectLaunchConfigurationChanges();
    function registerCommand(command, callback) {
        ctx.subscriptions.push(vscode_1.commands.registerCommand(command, callback));
    }
    let client = startServer(ctx);
    registerCommand("cadence.restartServer", () => __awaiter(this, void 0, void 0, function* () {
        if (!client) {
            return;
        }
        yield client.stop();
        client = startServer(ctx);
    }));
}
exports.activate = activate;
function startServer(ctx) {
    const languageServerCommand = vscode_1.workspace
        .getConfiguration("cadence")
        .get("languageServerCommand");
    if (!languageServerCommand) {
        vscode_1.window.showWarningMessage("Missing command to start the Cadence language server");
        return;
    }
    const startLanguageServerCommandAndArgs = languageServerCommand.split(/\s+/);
    if (startLanguageServerCommandAndArgs.length < 1) {
        vscode_1.window.showWarningMessage("Malformed language server command");
        return;
    }
    const command = startLanguageServerCommandAndArgs[0];
    const args = startLanguageServerCommandAndArgs.splice(1);
    const client = new vscode_languageclient_1.LanguageClient("cadence", "Cadence", {
        command,
        args,
    }, {
        documentSelector: [{ scheme: "file", language: "cadence" }],
        synchronize: {
            configurationSection: "cadence"
        }
    });
    client
        .onReady()
        .then(() => {
        return vscode_1.window.showInformationMessage("Cadence language server started");
    })
        .catch(error => {
        return vscode_1.window.showErrorMessage(`Cadence language server failed to start: ${error}`);
    });
    let languageServerDisposable = client.start();
    ctx.subscriptions.push(languageServerDisposable);
    return client;
}
function detectLaunchConfigurationChanges() {
    vscode_1.workspace.onDidChangeConfiguration(e => {
        const promptRestartKeys = ["languageServerPath"];
        const shouldPromptRestart = promptRestartKeys.some(key => e.affectsConfiguration(`cadence.${key}`));
        if (shouldPromptRestart) {
            vscode_1.window
                .showInformationMessage("Server launch configuration change detected. Reload the window for changes to take effect", "Reload Window", "Not now")
                .then(choice => {
                if (choice === "Reload Window") {
                    vscode_1.commands.executeCommand("workbench.action.reloadWindow");
                }
            });
        }
    });
}
function deactivate() { }
exports.deactivate = deactivate;
//# sourceMappingURL=extension.js.map