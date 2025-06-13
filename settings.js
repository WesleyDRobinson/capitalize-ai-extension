document.addEventListener('DOMContentLoaded', function() {
    const autoCapitalize = document.getElementById('autoCapitalize');
    const showNotification = document.getElementById('showNotification');
    const saveButton = document.getElementById('saveSettings');

    // Load saved settings
    chrome.storage.local.get(['autoCapitalize', 'showNotification'], function(result) {
        autoCapitalize.checked = result.autoCapitalize !== false;
        showNotification.checked = result.showNotification !== false;
    });

    // Save settings
    saveButton.addEventListener('click', function() {
        const settings = {
            autoCapitalize: autoCapitalize.checked,
            showNotification: showNotification.checked
        };

        chrome.storage.local.set(settings, function() {
            // Show save confirmation
            saveButton.textContent = 'Saved!';
            setTimeout(() => {
                saveButton.textContent = 'Save Settings';
            }, 2000);
        });
    });
}); 