import React from 'react';
import { LoadingDots } from '../common/Loading';

interface StreamingMessageProps {
  content: string;
  isComplete?: boolean;
}

export function StreamingMessage({ content, isComplete = false }: StreamingMessageProps) {
  return (
    <div className="flex justify-start">
      <div className="max-w-[80%] px-4 py-3 rounded-2xl bg-gray-100 text-gray-900">
        <div className="prose prose-sm max-w-none">
          {content ? (
            <p className="whitespace-pre-wrap">{content}</p>
          ) : (
            <p className="text-gray-500">
              Thinking<LoadingDots />
            </p>
          )}
          {!isComplete && content && (
            <span className="inline-block w-2 h-4 bg-primary-500 animate-pulse ml-1" />
          )}
        </div>
      </div>
    </div>
  );
}
