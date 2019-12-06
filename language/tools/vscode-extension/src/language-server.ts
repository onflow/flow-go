import {LanguageClient} from "vscode-languageclient";
import {ExtensionContext, Uri, window} from "vscode";
import {Config} from "./config";
import {CREATE_ACCOUNT_SERVER, CREATE_DEFAULT_ACCOUNTS_SERVER, SWITCH_ACCOUNT_SERVER} from "./commands";

// The args to pass to the Flow CLI to start the language server.
const START_LANGUAGE_SERVER_ARGS = ["cadence", "language-server"];

export class LanguageServerAPI {
    client: LanguageClient;

    constructor(ctx: ExtensionContext, config: Config) {
        this.client = new LanguageClient(
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

        this.client
            .onReady()
            .then(() => {
                return window.showInformationMessage("Cadence language server started");
            })
            .catch(err => {
                return window.showErrorMessage(
                    `Cadence language server failed to start: ${err}`
                );
            });

        const clientDisposable = this.client.start();
        ctx.subscriptions.push(clientDisposable);
    }

    // Sends a request to switch the currently active account.
    async switchActiveAccount(accountAddr: string) {
        return this.client.sendRequest("workspace/executeCommand", {
            command: SWITCH_ACCOUNT_SERVER,
            arguments: [
                accountAddr,
            ],
        });
    }

    // Sends a request to create a new account. Returns the address of the new
    // account, if it was created successfully.
    async createAccount(): Promise<string> {
        let res = await this.client.sendRequest("workspace/executeCommand", {
            command: CREATE_ACCOUNT_SERVER,
            arguments: [],
        });
        return res as string;
    }

    // Sends a request to create a new account. Returns the address of the new
    // account, if it was created successfully.
    async createDefaultAccounts(n: number): Promise<Array<string>> {
        let res = await this.client.sendRequest("workspace/executeCommand", {
            command: CREATE_DEFAULT_ACCOUNTS_SERVER,
            arguments: [n],
        });
        return res as Array<string>;
    }
}
