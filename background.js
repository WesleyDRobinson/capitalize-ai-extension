// Create context menu item
chrome.runtime.onInstalled.addListener(() => {
    chrome.contextMenus.create({
        id: "capitalizeSelection",
        title: "Capitalize 'ai' in selection",
        contexts: ["selection"]
    });
});

// Handle context menu clicks
chrome.contextMenus.onClicked.addListener((info, tab) => {
    if (info.menuItemId === "capitalizeSelection") {
        chrome.tabs.sendMessage(tab.id, {
            action: "capitalizeSelection",
            text: info.selectionText
        });
    }
});

// Handle keyboard shortcut
chrome.commands.onCommand.addListener((command) => {
    if (command === "capitalize-selection") {
        chrome.tabs.query({active: true, currentWindow: true}, (tabs) => {
            chrome.tabs.sendMessage(tabs[0].id, {
                action: "capitalizeSelection",
                text: null // Will get selection from content script
            });
        });
    }
}); 