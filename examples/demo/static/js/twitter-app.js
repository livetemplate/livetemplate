/**
 * Twitter Clone - Minimal Event Transmission (LiveTemplate Philosophy)
 * NO CLIENT-SIDE LOGIC - All UI state managed by server fragments
 */

let liveTemplateClient = null;

function initTwitterApp(client) {
    liveTemplateClient = client;
    console.log('[Twitter] Minimal JS initialized - server drives all UI');
    
    // Set up universal event delegation for ALL interactions
    document.addEventListener('click', handleAction);
    document.addEventListener('input', handleInput);
    document.addEventListener('keydown', handleKeydown);
}

// Universal action handler - sends raw event data to server
function handleAction(event) {
    const element = event.target.closest('[data-action]');
    if (!element) return;
    
    const action = element.dataset.action;
    const payload = extractEventData(element);
    
    // Send raw event to server - no client logic
    liveTemplateClient.sendAction(action, payload);
    
    console.log(`[Twitter] Sent ${action}:`, payload);
}

// Input handler for real-time updates (like character counting)
function handleInput(event) {
    const element = event.target;
    if (!element.dataset.action) return;
    
    const action = element.dataset.action;
    const payload = extractEventData(element);
    
    // Send every input to server for real-time fragment updates
    liveTemplateClient.sendAction(action, payload);
}

// Keyboard shortcuts (like Ctrl+Enter to tweet)
function handleKeydown(event) {
    if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
        const tweetBtn = document.getElementById('tweet-btn');
        if (tweetBtn && !tweetBtn.disabled) {
            event.preventDefault();
            liveTemplateClient.sendAction('tweet', {});
        }
    }
}

// Extract event data without any processing - raw data only
function extractEventData(element) {
    const data = {};
    
    // Extract data attributes
    Object.keys(element.dataset).forEach(key => {
        if (key !== 'action') {
            data[key.replace(/([A-Z])/g, '_$1').toLowerCase()] = element.dataset[key];
        }
    });
    
    // Extract form values
    if (element.value !== undefined) {
        data.value = element.value;
    }
    
    return data;
}

// That's it! <50 lines total.
// Server handles ALL logic: validation, state, UI updates, animations