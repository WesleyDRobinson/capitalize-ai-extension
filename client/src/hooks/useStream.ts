// hooks/useStream.ts
import { useEffect, useCallback, useRef } from 'react';
import { conversationStream } from '../lib/stream';
import { useAppStore } from '../store/useAppStore';
import { Message } from '../db';

interface UseStreamOptions {
  onMessage?: (message: Message) => void;
}

export function useStream(conversationId: string | null, options: UseStreamOptions = {}) {
  const { setConnected, addMessage } = useAppStore();
  const { onMessage } = options;
  const connectedRef = useRef(false);

  const connect = useCallback(async () => {
    if (!conversationId || connectedRef.current) return;

    try {
      await conversationStream.connect(conversationId, {
        onToken: () => {
          // Tokens are handled by useSendMessage during active streaming
        },
        onComplete: (message) => {
          addMessage(message);
          onMessage?.(message);
        },
        onError: (error) => {
          console.error('Stream error:', error);
          setConnected(false);
        },
        onConnected: () => {
          connectedRef.current = true;
          setConnected(true);
        },
        onDisconnected: () => {
          connectedRef.current = false;
          setConnected(false);
        }
      });
    } catch (error) {
      console.error('Failed to connect stream:', error);
      setConnected(false);
    }
  }, [conversationId, addMessage, onMessage, setConnected]);

  const disconnect = useCallback(() => {
    conversationStream.disconnect();
    connectedRef.current = false;
    setConnected(false);
  }, [setConnected]);

  useEffect(() => {
    if (conversationId) {
      connect();
    }

    return () => {
      disconnect();
    };
  }, [conversationId, connect, disconnect]);

  return {
    isConnected: connectedRef.current,
    connect,
    disconnect
  };
}
