
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { CommandPalette } from './CommandPalette';
import { SettingsModal } from './SettingsModal';
import { ExportModal } from './ExportModal';

export function Layout() {
  return (
    <div className="flex h-screen bg-gray-50 dark:bg-surface-950">
      <Sidebar />
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
      <CommandPalette />
      <SettingsModal />
      <ExportModal />
    </div>
  );
}
