import React from 'react';
import { useAppStore, Toast as ToastType } from '../../store/useAppStore';

const toastStyles: Record<ToastType['type'], { bg: string; border: string; icon: string }> = {
  error: {
    bg: 'bg-red-50',
    border: 'border-red-200',
    icon: '✕'
  },
  success: {
    bg: 'bg-green-50',
    border: 'border-green-200',
    icon: '✓'
  },
  warning: {
    bg: 'bg-yellow-50',
    border: 'border-yellow-200',
    icon: '⚠'
  },
  info: {
    bg: 'bg-blue-50',
    border: 'border-blue-200',
    icon: 'ℹ'
  }
};

const textColors: Record<ToastType['type'], string> = {
  error: 'text-red-800',
  success: 'text-green-800',
  warning: 'text-yellow-800',
  info: 'text-blue-800'
};

interface ToastItemProps {
  toast: ToastType;
  onDismiss: (id: string) => void;
}

function ToastItem({ toast, onDismiss }: ToastItemProps) {
  const style = toastStyles[toast.type];
  const textColor = textColors[toast.type];

  return (
    <div
      className={`flex items-start gap-3 p-4 rounded-lg border shadow-lg ${style.bg} ${style.border} animate-slide-in`}
      role="alert"
    >
      <span className={`text-lg ${textColor}`}>{style.icon}</span>
      <p className={`flex-1 text-sm font-medium ${textColor}`}>{toast.message}</p>
      <button
        onClick={() => onDismiss(toast.id)}
        className={`${textColor} hover:opacity-70 transition-opacity`}
        aria-label="Dismiss"
      >
        ✕
      </button>
    </div>
  );
}

export function ToastContainer() {
  const { toasts, removeToast } = useAppStore();

  if (toasts.length === 0) return null;

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 max-w-sm">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onDismiss={removeToast} />
      ))}
    </div>
  );
}

// Helper hook for easy toast usage
export function useToast() {
  const { addToast } = useAppStore();

  return {
    error: (message: string) => addToast('error', message),
    success: (message: string) => addToast('success', message),
    warning: (message: string) => addToast('warning', message),
    info: (message: string) => addToast('info', message)
  };
}
