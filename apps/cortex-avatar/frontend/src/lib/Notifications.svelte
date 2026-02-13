<script lang="ts">
  import { notifications, dismissNotification, type Notification } from '../stores/notifications';
  import { EventsOn } from '../../wailsjs/runtime/runtime';
  import { onMount } from 'svelte';

  // Wails binding for running commands
  declare const go: {
    bridge: {
      SettingsBridge: {
        RunCommand: (command: string) => Promise<{ success: boolean; output: string; error: string }>;
      };
    };
  };

  let runningCommands = new Set<string>();

  async function handleAction(notification: Notification) {
    if (!notification.action?.command) return;

    const id = notification.id;
    runningCommands.add(id);
    runningCommands = runningCommands;

    try {
      if (typeof go !== 'undefined' && go.bridge?.SettingsBridge?.RunCommand) {
        const result = await go.bridge.SettingsBridge.RunCommand(notification.action.command);
        if (result.success) {
          dismissNotification(id);
        } else {
          console.error('[Notifications] Command failed:', result.error);
        }
      } else {
        // Fallback: just show the command to copy
        console.log('[Notifications] Run this command:', notification.action.command);
        alert(`Run this command in terminal:\n\n${notification.action.command}`);
        dismissNotification(id);
      }
    } finally {
      runningCommands.delete(id);
      runningCommands = runningCommands;
    }
  }

  onMount(() => {
    // Listen for model missing events from Go backend
    EventsOn('cortex:model_missing', (data: { model: string }) => {
      import('../stores/notifications').then(({ notifyMissingModel }) => {
        notifyMissingModel(data.model);
      });
    });
  });
</script>

{#if $notifications.length > 0}
  <div class="notifications-container">
    {#each $notifications as notification (notification.id)}
      <div class="notification {notification.type}">
        <div class="notification-content">
          <div class="notification-header">
            <span class="notification-icon">
              {#if notification.type === 'error'}⚠️
              {:else if notification.type === 'warning'}⚡
              {:else if notification.type === 'action'}🔧
              {:else}ℹ️{/if}
            </span>
            <strong class="notification-title">{notification.title}</strong>
          </div>
          <p class="notification-message">{notification.message}</p>

          {#if notification.action}
            <div class="notification-actions">
              <button
                class="action-btn"
                on:click={() => handleAction(notification)}
                disabled={runningCommands.has(notification.id)}
              >
                {#if runningCommands.has(notification.id)}
                  Installing...
                {:else}
                  {notification.action.label}
                {/if}
              </button>
              {#if notification.action.command}
                <code class="command-hint">{notification.action.command}</code>
              {/if}
            </div>
          {/if}
        </div>

        {#if notification.dismissible !== false}
          <button
            class="dismiss-btn"
            on:click={() => dismissNotification(notification.id)}
            title="Dismiss"
          >×</button>
        {/if}
      </div>
    {/each}
  </div>
{/if}

<style>
  .notifications-container {
    position: fixed;
    top: 60px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 1000;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 90%;
    width: 360px;
  }

  .notification {
    display: flex;
    align-items: flex-start;
    padding: 12px 16px;
    border-radius: 12px;
    background: rgba(30, 30, 40, 0.95);
    border: 1px solid rgba(255, 255, 255, 0.1);
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
    animation: slideIn 0.3s ease;
  }

  .notification.error {
    border-color: rgba(255, 59, 48, 0.4);
    background: rgba(255, 59, 48, 0.1);
  }

  .notification.warning {
    border-color: rgba(255, 149, 0, 0.4);
    background: rgba(255, 149, 0, 0.1);
  }

  .notification.action {
    border-color: rgba(74, 158, 255, 0.4);
    background: rgba(74, 158, 255, 0.1);
  }

  .notification-content {
    flex: 1;
  }

  .notification-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 4px;
  }

  .notification-icon {
    font-size: 16px;
  }

  .notification-title {
    font-size: 14px;
    color: white;
  }

  .notification-message {
    font-size: 12px;
    color: rgba(255, 255, 255, 0.7);
    margin: 0 0 8px 0;
    line-height: 1.4;
  }

  .notification-actions {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .action-btn {
    padding: 8px 16px;
    border: none;
    border-radius: 6px;
    background: rgba(74, 158, 255, 0.8);
    color: white;
    font-size: 13px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .action-btn:hover:not(:disabled) {
    background: rgba(74, 158, 255, 1);
  }

  .action-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .command-hint {
    font-size: 10px;
    color: rgba(255, 255, 255, 0.4);
    background: rgba(0, 0, 0, 0.3);
    padding: 4px 8px;
    border-radius: 4px;
    word-break: break-all;
  }

  .dismiss-btn {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.5);
    font-size: 20px;
    cursor: pointer;
    padding: 0 0 0 8px;
    line-height: 1;
  }

  .dismiss-btn:hover {
    color: white;
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateY(-10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
