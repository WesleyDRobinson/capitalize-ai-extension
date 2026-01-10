import React from 'react';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';
import { useConversation } from '../../hooks/useConversation';
import { useSendMessage } from '../../hooks/useSendMessage';
import { useAppStore } from '../../store/useAppStore';
import { Loading } from '../common/Loading';

interface ChatContainerProps {
  conversationId: string;
}

export function ChatContainer({ conversationId }: ChatContainerProps) {
  const { messages, isLoading, isSyncing } = useConversation(conversationId);
  const sendMessage = useSendMessage(conversationId);
  const { streaming, isConnected } = useAppStore();

  const handleSend = (content: string) => {
    sendMessage.mutate(content);
  };

  const isCurrentConversationStreaming =
    streaming.isStreaming && streaming.streamingConversationId === conversationId;

  return (
    <div className="flex flex-col h-full bg-white">
      {/* Header */}
      <div className="border-b border-gray-200 px-4 py-3 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Conversation</h2>
          <div className="flex items-center space-x-2 text-sm text-gray-500">
            {isSyncing && (
              <span className="flex items-center">
                <Loading size="sm" className="mr-1" />
                Syncing...
              </span>
            )}
            <span className={`flex items-center ${isConnected ? 'text-green-500' : 'text-red-500'}`}>
              <span className={`w-2 h-2 rounded-full mr-1 ${isConnected ? 'bg-green-500' : 'bg-red-500'}`} />
              {isConnected ? 'Connected' : 'Disconnected'}
            </span>
          </div>
        </div>
      </div>

      {/* Messages */}
      {isLoading ? (
        <div className="flex-1 flex items-center justify-center">
          <Loading size="lg" />
        </div>
      ) : (
        <MessageList
          messages={messages}
          streamingContent={isCurrentConversationStreaming ? streaming.streamingContent : ''}
          isStreaming={isCurrentConversationStreaming}
        />
      )}

      {/* Input */}
      <MessageInput
        onSend={handleSend}
        disabled={sendMessage.isPending || isCurrentConversationStreaming}
        placeholder={
          isCurrentConversationStreaming
            ? 'Waiting for response...'
            : 'Type your message...'
        }
      />
    </div>
  );
}
