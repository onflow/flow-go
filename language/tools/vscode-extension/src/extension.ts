import {
  ExtensionContext,
  commands,
  workspace,
  window,
  ColorPresentation
} from "vscode";
import { LanguageClient } from "vscode-languageclient";
import { getConfig, Config } from "./config";

export function activate(ctx: ExtensionContext) {
  detectLaunchConfigurationChanges();

  function registerCommand(command: string, callback: (...args: any[]) => any) {
    ctx.subscriptions.push(commands.registerCommand(command, callback));
  }

  const maybeConfig: Config | undefined = getConfig();
  if (!maybeConfig) {
    window.showWarningMessage("Missing required config");
  }
  const config = maybeConfig as Config;

  let client = startServer(ctx, config);

  registerCommand("cadence.restartServer", async () => {
    if (!client) {
      return;
    }
    await client.stop();
    client = startServer(ctx, config);
  });
}

function startServer(ctx: ExtensionContext, config: Config): LanguageClient | undefined {
  const client = new LanguageClient(
    "cadence",
    "Cadence",
    {
      command: config.languageServerCommand,
      args: config.languageServerArgs,
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

  let languageServerDisposable = client.start();
  ctx.subscriptions.push(languageServerDisposable);

  return client;
}

function detectLaunchConfigurationChanges() {
  workspace.onDidChangeConfiguration(e => {
    // TODO: do something smarter for account/emulator config (re-send to server)
    const promptRestartKeys = ["languageServerPath", "accountKey", "accountAddress", "emulatorAddress"];
    const shouldPromptRestart = promptRestartKeys.some(key =>
      e.affectsConfiguration(`cadence.${key}`)
    );
    if (shouldPromptRestart) {
      window
        .showInformationMessage(
          "Server launch configuration change detected. Reload the window for changes to take effect",
          "Reload Window",
          "Not now"
        )
        .then(choice => {
          if (choice === "Reload Window") {
            commands.executeCommand("workbench.action.reloadWindow");
          }
        });
    }
  });
}

export function deactivate() {}
