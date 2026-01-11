import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './styles/globals.css';
import { retryFailedMessages } from './hooks/useSendMessage';

// Initialize offline resilience: retry failed messages
// Run immediately on app start
retryFailedMessages().catch(console.error);

// Retry failed messages periodically (every 30 seconds)
const RETRY_INTERVAL_MS = 30_000;
setInterval(() => {
  retryFailedMessages().catch(console.error);
}, RETRY_INTERVAL_MS);

// Retry when coming back online
window.addEventListener('online', () => {
  console.log('Network reconnected, retrying failed messages...');
  retryFailedMessages().catch(console.error);
});

// Log when going offline
window.addEventListener('offline', () => {
  console.log('Network disconnected, messages will be queued for retry');
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
