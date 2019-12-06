import {
    window,
    StatusBarItem,
    StatusBarAlignment,
} from "vscode";
import {Extension} from "./extension";
import {SWITCH_ACCOUNT} from "./commands";

export function createActiveAccountStatusBarItem(): StatusBarItem {
    const statusBarItem = window.createStatusBarItem(StatusBarAlignment.Left, 100);
    statusBarItem.command = SWITCH_ACCOUNT;
    return statusBarItem
}

export function updateActiveAccountStatusBarItem(statusBarItem: StatusBarItem, activeAccount: string): void {
    statusBarItem.text = `$(key) Active account: ${activeAccount}`
    statusBarItem.show()
}
