import { writable } from 'svelte/store';

export interface Notification {
  id: string;
  type: 'info' | 'warning' | 'error' | 'action';
  title: string;
  message: string;
  action?: {
    label: string;
    command?: string;  // Shell command to run
  };
  dismissible?: boolean;
  timeout?: number;  // Auto-dismiss after ms
}

export const notifications = writable<Notification[]>([]);

let notificationId = 0;

export function addNotification(notification: Omit<Notification, 'id'>): string {
  const id = `notification-${++notificationId}`;
  const newNotification: Notification = { ...notification, id };

  notifications.update(n => [...n, newNotification]);

  // Auto-dismiss if timeout is set
  if (notification.timeout) {
    setTimeout(() => {
      dismissNotification(id);
    }, notification.timeout);
  }

  return id;
}

export function dismissNotification(id: string): void {
  notifications.update(n => n.filter(notif => notif.id !== id));
}

export function clearAllNotifications(): void {
  notifications.set([]);
}

// Helper for missing model notifications
export function notifyMissingModel(modelName: string): void {
  addNotification({
    type: 'action',
    title: 'Missing Ollama Model',
    message: `CortexBrain requires the model "${modelName}" which is not installed.`,
    action: {
      label: `Install ${modelName}`,
      command: `ollama pull ${modelName}`,
    },
    dismissible: true,
  });
}
