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
  const connectedRef = useRef(false);

  // Store callbacks in refs to avoid recreating effect dependencies
  const onMessageRef = useRef(options.onMessage);
  onMessageRef.current = options.onMessage;

  const addMessageRef = useRef(addMessage);
  addMessageRef.current = addMessage;

  const setConnectedRef = useRef(setConnected);
  setConnectedRef.current = setConnected;

  // Stable connect function that only depends on conversationId
  const connect = useCallback(async (convId: string) => {
    if (connectedRef.current) return;

    try {
      await conversationStream.connect(convId, {
        onToken: () => {
          // Tokens are handled by useSendMessage during active streaming
        },
        onComplete: (message) => {
          addMessageRef.current(message);
          onMessageRef.current?.(message);
        },
        onError: (error) => {
          console.error('Stream error:', error);
          connectedRef.current = false;
          setConnectedRef.current(false);
        },
        onConnected: () => {
          connectedRef.current = true;
          setConnectedRef.current(true);
        },
        onDisconnected: () => {
          connectedRef.current = false;
          setConnectedRef.current(false);
        }
      });
    } catch (error) {
      console.error('Failed to connect stream:', error);
      connectedRef.current = false;
      setConnectedRef.current(false);
    }
  }, []); // No dependencies - uses refs for all callbacks

  const disconnect = useCallback(() => {
    conversationStream.disconnect();
    connectedRef.current = false;
    setConnectedRef.current(false);
  }, []); // No dependencies - uses refs

  // Effect only runs when conversationId changes
  useEffect(() => {
    if (conversationId) {
      connect(conversationId);
    }

    return () => {
      disconnect();
    };
  }, [conversationId, connect, disconnect]);

  return {
    isConnected: connectedRef.current,
    connect: () => conversationId && connect(conversationId),
    disconnect
  };
}
