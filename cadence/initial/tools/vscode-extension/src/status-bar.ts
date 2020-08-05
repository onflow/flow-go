import {
    window,
    StatusBarItem,
    StatusBarAlignment,
} from "vscode";
import {Account} from "./config";
import {SWITCH_ACCOUNT} from "./commands";

export function createActiveAccountStatusBarItem(): StatusBarItem {
    const statusBarItem = window.createStatusBarItem(StatusBarAlignment.Left, 100);
    statusBarItem.command = SWITCH_ACCOUNT;
    return statusBarItem
}

export function updateActiveAccountStatusBarItem(statusBarItem: StatusBarItem, activeAccount: Account): void {
    statusBarItem.text = `$(key) Active account: ${activeAccount.fullName()}`
    statusBarItem.show()
}
