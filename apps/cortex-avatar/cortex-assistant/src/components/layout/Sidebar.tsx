
import { NavLink } from 'react-router-dom';
import { clsx } from 'clsx';
import {
  Home,
  History,
  ListTodo,
  Brain,
  Settings,
  ChevronLeft,
  Mic,
} from 'lucide-react';
import { useUIStore, useMeetingStore, useCortexStore } from '@/store';
import { Button } from '@/components/ui';

const navItems = [
  { to: '/', icon: Home, label: 'Dashboard' },
  { to: '/history', icon: History, label: 'History' },
  { to: '/tasks', icon: ListTodo, label: 'Tasks' },
  { to: '/memory', icon: Brain, label: 'Memory' },
];

export function Sidebar() {
  const { sidebarOpen, toggleSidebar, openSettings } = useUIStore();
  const { currentMeeting, recordingStatus } = useMeetingStore();
  const { status } = useCortexStore();

  return (
    <aside
      className={clsx(
        'flex flex-col bg-white dark:bg-surface-900 border-r border-gray-200 dark:border-surface-700 transition-all duration-200',
        sidebarOpen ? 'w-64' : 'w-16'
      )}
    >
      <div className="flex items-center justify-between h-14 px-4 border-b border-gray-200 dark:border-surface-700">
        {sidebarOpen && (
          <div className="flex items-center gap-2">
            <Brain className="w-6 h-6 text-primary-500" />
            <span className="font-semibold text-gray-900 dark:text-white">
              Cortex
            </span>
          </div>
        )}
        <Button
          variant="ghost"
          size="sm"
          onClick={toggleSidebar}
          className={clsx(!sidebarOpen && 'mx-auto')}
        >
          <ChevronLeft
            className={clsx(
              'w-4 h-4 transition-transform',
              !sidebarOpen && 'rotate-180'
            )}
          />
        </Button>
      </div>

      <nav className="flex-1 p-2 space-y-1">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              clsx(
                'flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium transition-colors',
                isActive
                  ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-surface-800'
              )
            }
          >
            <item.icon className="w-5 h-5 shrink-0" />
            {sidebarOpen && <span>{item.label}</span>}
          </NavLink>
        ))}
      </nav>

      <div className="p-2 border-t border-gray-200 dark:border-surface-700">
        {currentMeeting && recordingStatus === 'recording' && (
          <div
            className={clsx(
              'flex items-center gap-2 px-3 py-2 mb-2 rounded-lg bg-red-50 dark:bg-red-900/20',
              !sidebarOpen && 'justify-center'
            )}
          >
            <div className="relative">
              <Mic className="w-4 h-4 text-red-500" />
              <div className="absolute -top-0.5 -right-0.5 w-2 h-2 bg-red-500 rounded-full animate-pulse" />
            </div>
            {sidebarOpen && (
              <span className="text-xs font-medium text-red-600 dark:text-red-400">
                Recording...
              </span>
            )}
          </div>
        )}

        <div
          className={clsx(
            'flex items-center gap-2 px-3 py-2 rounded-lg',
            !sidebarOpen && 'justify-center'
          )}
        >
          <div
            className={clsx(
              'w-2 h-2 rounded-full',
              status === 'connected' && 'bg-green-500',
              status === 'connecting' && 'bg-yellow-500 animate-pulse',
              status === 'degraded' && 'bg-yellow-500',
              status === 'offline' && 'bg-red-500'
            )}
          />
          {sidebarOpen && (
            <span className="text-xs text-gray-500 dark:text-gray-400">
              Cortex: {status}
            </span>
          )}
        </div>

        <Button
          variant="ghost"
          className={clsx('w-full justify-start', !sidebarOpen && 'justify-center')}
          onClick={openSettings}
        >
          <Settings className="w-5 h-5" />
          {sidebarOpen && <span className="ml-2">Settings</span>}
        </Button>
      </div>
    </aside>
  );
}
