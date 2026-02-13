import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { ActionItem, Priority, TaskStatus } from '@/models';
import { generateId } from '@/models';

interface TaskFilters {
  status: TaskStatus | 'all';
  priority: Priority | 'all';
  assignee: string | 'all';
  meetingId: string | 'all';
}

interface TasksState {
  tasks: ActionItem[];
  filters: TaskFilters;

  addTask: (task: Omit<ActionItem, 'id' | 'createdAt' | 'updatedAt'>) => ActionItem;
  updateTask: (id: string, updates: Partial<ActionItem>) => void;
  deleteTask: (id: string) => void;
  completeTask: (id: string) => void;
  reopenTask: (id: string) => void;
  setFilter: <K extends keyof TaskFilters>(key: K, value: TaskFilters[K]) => void;
  clearFilters: () => void;
  getFilteredTasks: () => ActionItem[];
  getTasksByMeeting: (meetingId: string) => ActionItem[];
  importTasks: (tasks: ActionItem[]) => void;
  getTaskStats: () => {
    total: number;
    pending: number;
    inProgress: number;
    completed: number;
    overdue: number;
  };
}

const DEFAULT_FILTERS: TaskFilters = {
  status: 'all',
  priority: 'all',
  assignee: 'all',
  meetingId: 'all',
};

export const useTasksStore = create<TasksState>()(
  persist(
    (set, get) => ({
      tasks: [],
      filters: DEFAULT_FILTERS,

      addTask: (taskData) => {
        const now = new Date().toISOString();
        const task: ActionItem = {
          ...taskData,
          id: generateId(),
          createdAt: now,
          updatedAt: now,
        };
        set((state) => ({
          tasks: [...state.tasks, task],
        }));
        return task;
      },

      updateTask: (id, updates) => {
        set((state) => ({
          tasks: state.tasks.map((task) =>
            task.id === id
              ? { ...task, ...updates, updatedAt: new Date().toISOString() }
              : task
          ),
        }));
      },

      deleteTask: (id) => {
        set((state) => ({
          tasks: state.tasks.filter((task) => task.id !== id),
        }));
      },

      completeTask: (id) => {
        set((state) => ({
          tasks: state.tasks.map((task) =>
            task.id === id
              ? { ...task, status: 'completed' as TaskStatus, updatedAt: new Date().toISOString() }
              : task
          ),
        }));
      },

      reopenTask: (id) => {
        set((state) => ({
          tasks: state.tasks.map((task) =>
            task.id === id
              ? { ...task, status: 'pending' as TaskStatus, updatedAt: new Date().toISOString() }
              : task
          ),
        }));
      },

      setFilter: (key, value) => {
        set((state) => ({
          filters: { ...state.filters, [key]: value },
        }));
      },

      clearFilters: () => {
        set({ filters: DEFAULT_FILTERS });
      },

      getFilteredTasks: () => {
        const { tasks, filters } = get();
        return tasks.filter((task) => {
          if (filters.status !== 'all' && task.status !== filters.status) return false;
          if (filters.priority !== 'all' && task.priority !== filters.priority) return false;
          if (filters.assignee !== 'all' && task.assignee !== filters.assignee) return false;
          if (filters.meetingId !== 'all' && task.meetingId !== filters.meetingId) return false;
          return true;
        });
      },

      getTasksByMeeting: (meetingId) => {
        return get().tasks.filter((task) => task.meetingId === meetingId);
      },

      importTasks: (newTasks) => {
        set((state) => {
          const existingIds = new Set(state.tasks.map((t) => t.id));
          const tasksToAdd = newTasks.filter((t) => !existingIds.has(t.id));
          return {
            tasks: [...state.tasks, ...tasksToAdd],
          };
        });
      },

      getTaskStats: () => {
        const tasks = get().tasks;
        const now = new Date();
        return {
          total: tasks.length,
          pending: tasks.filter((t) => t.status === 'pending').length,
          inProgress: tasks.filter((t) => t.status === 'in_progress').length,
          completed: tasks.filter((t) => t.status === 'completed').length,
          overdue: tasks.filter(
            (t) =>
              t.status !== 'completed' &&
              t.status !== 'cancelled' &&
              t.dueDate &&
              new Date(t.dueDate) < now
          ).length,
        };
      },
    }),
    {
      name: 'cortex-assistant-tasks',
    }
  )
);
