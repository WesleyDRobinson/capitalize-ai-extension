import React, { useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ConversationList } from './components/Sidebar/ConversationList';
import { ChatContainer } from './components/Chat/ChatContainer';
import { ToastContainer } from './components/common/Toast';
import { useAppStore } from './store/useAppStore';
import { isAuthenticated, setAccessToken, generateDemoToken } from './lib/auth';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      refetchOnWindowFocus: false
    }
  }
});

function AppContent() {
  const { currentConversationId, sidebarOpen, toggleSidebar, setAuthenticated } = useAppStore();

  // Initialize auth (demo mode for development)
  useEffect(() => {
    if (!isAuthenticated()) {
      // Generate demo token for development
      const token = generateDemoToken();
      setAccessToken(token);
    }
    setAuthenticated(true);
  }, [setAuthenticated]);

  return (
    <div className="flex h-screen bg-gray-50">
      {/* Sidebar */}
      <aside
        className={`
          ${sidebarOpen ? 'w-80' : 'w-0'}
          transition-all duration-300 ease-in-out
          bg-white border-r border-gray-200 overflow-hidden
          flex-shrink-0
        `}
      >
        <div className="w-80 h-full">
          <ConversationList />
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 flex flex-col min-w-0">
        {/* Toggle sidebar button */}
        <button
          onClick={toggleSidebar}
          className="absolute top-4 left-4 z-10 p-2 rounded-lg bg-white shadow-md hover:bg-gray-50 transition-colors"
          title={sidebarOpen ? 'Hide sidebar' : 'Show sidebar'}
        >
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="currentColor"
            className="w-5 h-5 text-gray-600"
          >
            {sidebarOpen ? (
              <path
                fillRule="evenodd"
                d="M3 6.75A.75.75 0 013.75 6h16.5a.75.75 0 010 1.5H3.75A.75.75 0 013 6.75zM3 12a.75.75 0 01.75-.75h16.5a.75.75 0 010 1.5H3.75A.75.75 0 013 12zm0 5.25a.75.75 0 01.75-.75h16.5a.75.75 0 010 1.5H3.75a.75.75 0 01-.75-.75z"
                clipRule="evenodd"
              />
            ) : (
              <path
                fillRule="evenodd"
                d="M2.25 12c0-5.385 4.365-9.75 9.75-9.75s9.75 4.365 9.75 9.75-4.365 9.75-9.75 9.75S2.25 17.385 2.25 12zm14.024-.983a1.125 1.125 0 010 1.966l-5.603 3.113A1.125 1.125 0 019 15.113V8.887c0-.857.921-1.4 1.671-.983l5.603 3.113z"
                clipRule="evenodd"
              />
            )}
          </svg>
        </button>

        {/* Chat or welcome screen */}
        {currentConversationId ? (
          <ChatContainer conversationId={currentConversationId} />
        ) : (
          <WelcomeScreen />
        )}
      </main>
    </div>
  );
}

function WelcomeScreen() {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-center max-w-md px-4">
        <div className="mx-auto w-16 h-16 bg-primary-100 rounded-full flex items-center justify-center mb-6">
          <svg
            className="w-8 h-8 text-primary-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"
            />
          </svg>
        </div>
        <h1 className="text-2xl font-bold text-gray-900 mb-2">
          Conversational AI Platform
        </h1>
        <p className="text-gray-600 mb-6">
          Real-time AI conversations with NATS JetStream persistence,
          offline support, and seamless synchronization.
        </p>
        <div className="grid grid-cols-2 gap-4 text-sm text-left">
          <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-100">
            <h3 className="font-semibold text-gray-900 mb-1">Real-time Streaming</h3>
            <p className="text-gray-500">Sub-100ms token delivery via SSE</p>
          </div>
          <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-100">
            <h3 className="font-semibold text-gray-900 mb-1">Durable Storage</h3>
            <p className="text-gray-500">Messages persisted in JetStream</p>
          </div>
          <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-100">
            <h3 className="font-semibold text-gray-900 mb-1">Offline Support</h3>
            <p className="text-gray-500">IndexedDB caching with Dexie.js</p>
          </div>
          <div className="bg-white p-4 rounded-lg shadow-sm border border-gray-100">
            <h3 className="font-semibold text-gray-900 mb-1">Auto Sync</h3>
            <p className="text-gray-500">Seamless reconnection & replay</p>
          </div>
        </div>
        <p className="text-sm text-gray-400 mt-6">
          Select or create a conversation to begin
        </p>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AppContent />
      <ToastContainer />
    </QueryClientProvider>
  );
}
