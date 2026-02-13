import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Layout } from '@/components/layout';
import { Dashboard, MeetingView, HistoryView, TasksView, MemoryView } from '@/views';
import { useTheme, useKeyboardShortcuts, useCortexConnection } from '@/hooks';

export function App() {
  useTheme();
  useKeyboardShortcuts();
  useCortexConnection();

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Dashboard />} />
          <Route path="meeting" element={<MeetingView />} />
          <Route path="history" element={<HistoryView />} />
          <Route path="tasks" element={<TasksView />} />
          <Route path="memory" element={<MemoryView />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
