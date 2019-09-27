import {
  ExtensionContext,
  commands,
  workspace,
  window,
  ColorPresentation
} from "vscode";
import { LanguageClient } from "vscode-languageclient";

export function activate(ctx: ExtensionContext) {
  detectLaunchConfigurationChanges();

  function registerCommand(command: string, callback: (...args: any[]) => any) {
    ctx.subscriptions.push(commands.registerCommand(command, callback));
  }

  let client = startServer(ctx);

  registerCommand("bamboo.restartServer", async () => {
    if (!client) {
      return;
    }
    await client.stop();
    client = startServer(ctx);
  });
}

function startServer(ctx: ExtensionContext): LanguageClient | undefined {
  const serverBinaryPath: string | undefined = workspace
    .getConfiguration("bamboo")
    .get("languageServerPath");

  if (!serverBinaryPath) {
    window.showWarningMessage("Missing path to Bamboo language server");
    return;
  }

  const client = new LanguageClient(
    "bamboo",
    "Bamboo",
    {
      command: serverBinaryPath
    },
    {
      documentSelector: [{ scheme: "file", language: "bamboo" }],
      synchronize: {
        configurationSection: "bamboo"
      }
    }
  );

  client
    .onReady()
    .then(() => {
      return window.showInformationMessage("Bamboo language server started");
    })
    .catch(error => {
      return window.showErrorMessage(
        `Bamboo language server failed to start: ${error}`
      );
    });

  let languageServerDisposable = client.start();
  ctx.subscriptions.push(languageServerDisposable);

  return client;
}

function detectLaunchConfigurationChanges() {
  workspace.onDidChangeConfiguration(e => {
    const promptRestartKeys = ["languageServerPath"];
    const shouldPromptRestart = promptRestartKeys.some(key =>
      e.affectsConfiguration(`bamboo.${key}`)
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
