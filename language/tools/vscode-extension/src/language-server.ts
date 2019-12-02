import {LanguageClient} from "vscode-languageclient";
import {window} from "vscode";
import {Extension} from "./extension";

// The args to pass to the Flow CLI to start the language server.
const START_LANGUAGE_SERVER_ARGS = ["cadence", "language-server"];

// Starts the language server and returns a client object.
export function startServer(ext: Extension): LanguageClient | undefined {
    const client = new LanguageClient(
        "cadence",
        "Cadence",
        {
            command: ext.config.flowCommand,
            args: START_LANGUAGE_SERVER_ARGS,
        },
        {
            documentSelector: [{ scheme: "file", language: "cadence" }],
            synchronize: {
                configurationSection: "cadence"
            },
            initializationOptions: ext.config.serverConfig,
        }
    );

    client
        .onReady()
        .then(() => {
            return window.showInformationMessage("Cadence language server started");
        })
        .catch(error => {
            return window.showErrorMessage(
                `Cadence language server failed to start: ${error}`
            );
        });

    let languageServerDisposable = client.start();
    ext.ctx.subscriptions.push(languageServerDisposable);

    return client;
}
