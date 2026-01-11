import React, { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ConversationItem } from './ConversationItem';
import { Button } from '../common/Button';
import { Input } from '../common/Input';
import { Loading } from '../common/Loading';
import { useConversations } from '../../hooks/useConversation';
import { useAppStore } from '../../store/useAppStore';
import { createConversation, deleteConversation } from '../../api/conversations';

export function ConversationList() {
  const [showNewForm, setShowNewForm] = useState(false);
  const [newTitle, setNewTitle] = useState('');
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useConversations();
  const {
    currentConversationId,
    setCurrentConversation,
    addConversation,
    removeConversation
  } = useAppStore();

  const createMutation = useMutation({
    mutationFn: (title: string) => createConversation({ title }),
    onSuccess: (conversation) => {
      addConversation(conversation);
      setCurrentConversation(conversation.id);
      setShowNewForm(false);
      setNewTitle('');
      queryClient.invalidateQueries({ queryKey: ['conversations'] });
    }
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => deleteConversation(id),
    onSuccess: (_, id) => {
      removeConversation(id);
      queryClient.invalidateQueries({ queryKey: ['conversations'] });
    }
  });

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault();
    if (newTitle.trim()) {
      createMutation.mutate(newTitle.trim());
    }
  };

  const conversations = data?.conversations ?? [];

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="p-4 border-b border-gray-200">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Conversations</h2>
          <Button
            size="sm"
            onClick={() => setShowNewForm(!showNewForm)}
          >
            {showNewForm ? 'Cancel' : 'New'}
          </Button>
        </div>

        {/* New conversation form */}
        {showNewForm && (
          <form onSubmit={handleCreate} className="mt-3">
            <Input
              value={newTitle}
              onChange={(e) => setNewTitle(e.target.value)}
              placeholder="Conversation title..."
              autoFocus
            />
            <Button
              type="submit"
              className="w-full mt-2"
              loading={createMutation.isPending}
              disabled={!newTitle.trim()}
            >
              Create
            </Button>
          </form>
        )}
      </div>

      {/* Conversation list */}
      <div className="flex-1 overflow-y-auto p-2">
        {isLoading ? (
          <div className="flex justify-center py-8">
            <Loading />
          </div>
        ) : error ? (
          <div className="text-center py-8 text-red-500">
            <p>Failed to load conversations</p>
          </div>
        ) : conversations.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            <p className="text-sm">No conversations yet</p>
            <p className="text-xs mt-1">Click "New" to start one</p>
          </div>
        ) : (
          <div className="space-y-1">
            {conversations.map((conversation) => (
              <ConversationItem
                key={conversation.id}
                conversation={conversation}
                isActive={conversation.id === currentConversationId}
                onClick={() => setCurrentConversation(conversation.id)}
                onDelete={() => deleteMutation.mutate(conversation.id)}
              />
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      <div className="p-4 border-t border-gray-200">
        <p className="text-xs text-gray-400 text-center">
          Powered by NATS JetStream
        </p>
      </div>
    </div>
  );
}
