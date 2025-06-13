// State management
let isEnabled = true;
let wordCount = 0;
let showNotification = true;

// Function to show notification
function showCapitalizationNotification(count) {
    if (!showNotification) return;
    
    const notification = document.createElement('div');
    notification.style.cssText = `
        position: fixed;
        bottom: 20px;
        right: 20px;
        background-color: #4285f4;
        color: white;
        padding: 10px 20px;
        border-radius: 5px;
        font-family: Arial, sans-serif;
        z-index: 10000;
        box-shadow: 0 2px 5px rgba(0,0,0,0.2);
    `;
    notification.textContent = `Capitalized ${count} instance${count === 1 ? '' : 's'} of 'ai'`;
    document.body.appendChild(notification);
    
    setTimeout(() => {
        notification.style.opacity = '0';
        notification.style.transition = 'opacity 0.5s';
        setTimeout(() => notification.remove(), 500);
    }, 2000);
}

// Function to update statistics
function updateStats(count) {
    wordCount += count;
    const now = new Date().toLocaleString();
    chrome.storage.local.set({
        wordCount: wordCount,
        lastUpdated: now
    });
    showCapitalizationNotification(count);
}

// Function to capitalize text
function capitalizeText(text) {
    return text.replace(/ai/gi, 'AI');
}

// Function to capitalize instances of 'ai'
function capitalizeAI() {
    if (!isEnabled) return;

    // Get all text nodes in the document
    const walker = document.createTreeWalker(
        document.body,
        NodeFilter.SHOW_TEXT,
        {
            acceptNode: function(node) {
                // Skip script and style tags
                if (node.parentElement.tagName === 'SCRIPT' || 
                    node.parentElement.tagName === 'STYLE') {
                    return NodeFilter.FILTER_REJECT;
                }
                return NodeFilter.FILTER_ACCEPT;
            }
        },
        false
    );

    const nodesToReplace = [];
    let node;
    let totalReplacements = 0;
    
    // Find all text nodes containing 'ai'
    while (node = walker.nextNode()) {
        if (node.nodeValue.toLowerCase().includes('ai')) {
            nodesToReplace.push(node);
        }
    }

    // Replace 'ai' with 'AI' in each found node
    nodesToReplace.forEach(node => {
        const originalText = node.nodeValue;
        const newText = capitalizeText(originalText);
        if (originalText !== newText) {
            node.nodeValue = newText;
            totalReplacements += (newText.match(/AI/g) || []).length;
        }
    });

    if (totalReplacements > 0) {
        updateStats(totalReplacements);
    }
}

// Function to capitalize selected text
function capitalizeSelection(text) {
    if (!text) {
        const selection = window.getSelection();
        if (!selection.rangeCount) return;
        
        const range = selection.getRangeAt(0);
        const selectedText = range.toString();
        const capitalizedText = capitalizeText(selectedText);
        
        if (selectedText !== capitalizedText) {
            document.execCommand('insertText', false, capitalizedText);
            updateStats((capitalizedText.match(/AI/g) || []).length);
        }
    } else {
        const capitalizedText = capitalizeText(text);
        if (text !== capitalizedText) {
            document.execCommand('insertText', false, capitalizedText);
            updateStats((capitalizedText.match(/AI/g) || []).length);
        }
    }
}

// Add debounce function
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Function to handle dynamic content
function handleDynamicContent(mutations) {
    let shouldProcess = false;
    
    mutations.forEach(mutation => {
        if (mutation.addedNodes.length) {
            shouldProcess = true;
        }
    });

    if (shouldProcess) {
        debouncedCapitalizeAI();
    }
}

// Create debounced version of capitalizeAI
const debouncedCapitalizeAI = debounce(capitalizeAI, 250);

// Initialize the extension
function initialize() {
    // Load saved state
    chrome.storage.local.get(['enabled', 'showNotification'], function(result) {
        isEnabled = result.enabled !== false;
        showNotification = result.showNotification !== false;
    });

    // Run initial capitalization
    capitalizeAI();

    // Set up MutationObserver for dynamic content
    const observer = new MutationObserver(handleDynamicContent);
    observer.observe(document.body, {
        childList: true,
        subtree: true,
        characterData: true
    });
}

// Listen for messages from popup and background script
chrome.runtime.onMessage.addListener((request, sender, sendResponse) => {
    if (request.action === 'toggleCapitalization') {
        isEnabled = request.enabled;
        if (isEnabled) {
            capitalizeAI();
        }
    } else if (request.action === 'capitalizeSelection') {
        capitalizeSelection(request.text);
    }
});

// Initialize when the page is ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initialize);
} else {
    initialize();
}

// Add error handling wrapper
function safeExecute(fn) {
    return function(...args) {
        try {
            return fn.apply(this, args);
        } catch (error) {
            console.error('AI Capitalizer Error:', error);
            // Show user-friendly error notification
            showCapitalizationNotification('Error occurred. Please try again.');
        }
    };
}

// Wrap main functions with error handling
capitalizeAI = safeExecute(capitalizeAI);
capitalizeSelection = safeExecute(capitalizeSelection);
handleDynamicContent = safeExecute(handleDynamicContent); 