import {LanguageClient} from "vscode-languageclient";
import {ExtensionContext, window} from "vscode";
import {Config} from "./config";

// The args to pass to the Flow CLI to start the language server.
const START_LANGUAGE_SERVER_ARGS = ["cadence", "language-server"];

// Starts the language server and returns a client object.
export function startServer(ctx: ExtensionContext, config: Config): LanguageClient {
    const client = new LanguageClient(
        "cadence",
        "Cadence",
        {
            command: config.flowCommand,
            args: START_LANGUAGE_SERVER_ARGS,
        },
        {
            documentSelector: [{ scheme: "file", language: "cadence" }],
            synchronize: {
                configurationSection: "cadence"
            },
            initializationOptions: config.serverConfig,
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

    const clientDisposable = client.start();
    ctx.subscriptions.push(clientDisposable);

    return client;
}
