document.addEventListener('DOMContentLoaded', function() {
    const toggleSwitch = document.getElementById('toggleExtension');
    const wordCountElement = document.getElementById('wordCount');
    const lastUpdatedElement = document.getElementById('lastUpdated');

    // Load saved state
    chrome.storage.local.get(['enabled', 'wordCount', 'lastUpdated'], function(result) {
        toggleSwitch.checked = result.enabled !== false; // Default to true if not set
        wordCountElement.textContent = result.wordCount || '0';
        lastUpdatedElement.textContent = result.lastUpdated || 'Never';
    });

    // Handle toggle switch
    toggleSwitch.addEventListener('change', function() {
        const enabled = toggleSwitch.checked;
        
        // Save state
        chrome.storage.local.set({ enabled: enabled });

        // Send message to content script
        chrome.tabs.query({active: true, currentWindow: true}, function(tabs) {
            chrome.tabs.sendMessage(tabs[0].id, {
                action: 'toggleCapitalization',
                enabled: enabled
            });
        });
    });
}); 