import React, { useState } from 'react';
import { clsx } from 'clsx';
import {
  Plus,
  Check,
  Clock,
  AlertCircle,
  Trash2,
  Edit2,
  X,
} from 'lucide-react';
import { useTasksStore } from '@/store';
import { formatDate } from '@/utils/format';
import { Button, Input, Card, Badge, Select, Modal } from '@/components/ui';
import type { ActionItem, Priority, TaskStatus } from '@/models';

export function TasksView() {
  const {
    filters,
    setFilter,
    clearFilters,
    getFilteredTasks,
    addTask,
    updateTask,
    deleteTask,
    completeTask,
    reopenTask,
  } = useTasksStore();

  const [isAddingTask, setIsAddingTask] = useState(false);
  const [editingTask, setEditingTask] = useState<ActionItem | null>(null);
  const [newTask, setNewTask] = useState({
    text: '',
    assignee: '',
    dueDate: '',
    priority: 'medium' as Priority,
  });

  const tasks = getFilteredTasks();

  const handleAddTask = () => {
    if (!newTask.text.trim()) return;

    addTask({
      text: newTask.text,
      assignee: newTask.assignee || undefined,
      dueDate: newTask.dueDate || undefined,
      priority: newTask.priority,
      status: 'pending',
      sourceSegmentIds: [],
      meetingId: 'manual',
    });

    setNewTask({ text: '', assignee: '', dueDate: '', priority: 'medium' });
    setIsAddingTask(false);
  };

  const handleUpdateTask = () => {
    if (!editingTask) return;

    updateTask(editingTask.id, {
      text: editingTask.text,
      assignee: editingTask.assignee,
      dueDate: editingTask.dueDate,
      priority: editingTask.priority,
    });

    setEditingTask(null);
  };

  const handleToggleComplete = (task: ActionItem) => {
    if (task.status === 'completed') {
      reopenTask(task.id);
    } else {
      completeTask(task.id);
    }
  };

  const groupedTasks = {
    overdue: tasks.filter(
      (t) =>
        t.status !== 'completed' &&
        t.dueDate &&
        new Date(t.dueDate) < new Date()
    ),
    today: tasks.filter(
      (t) =>
        t.status !== 'completed' &&
        t.dueDate &&
        new Date(t.dueDate).toDateString() === new Date().toDateString()
    ),
    upcoming: tasks.filter(
      (t) =>
        t.status !== 'completed' &&
        t.dueDate &&
        new Date(t.dueDate) > new Date() &&
        new Date(t.dueDate).toDateString() !== new Date().toDateString()
    ),
    noDue: tasks.filter((t) => t.status !== 'completed' && !t.dueDate),
    completed: tasks.filter((t) => t.status === 'completed'),
  };

  const TaskItem = ({ task }: { task: ActionItem }) => {
    const isOverdue =
      task.status !== 'completed' &&
      task.dueDate &&
      new Date(task.dueDate) < new Date();

    return (
      <div
        className={clsx(
          'flex items-start gap-3 p-3 rounded-lg group transition-colors',
          task.status === 'completed'
            ? 'bg-gray-50 dark:bg-surface-800/50'
            : 'bg-white dark:bg-surface-800 hover:bg-gray-50 dark:hover:bg-surface-700'
        )}
      >
        <button
          onClick={() => handleToggleComplete(task)}
          className={clsx(
            'w-5 h-5 rounded-full border-2 flex items-center justify-center shrink-0 mt-0.5 transition-colors',
            task.status === 'completed'
              ? 'bg-green-500 border-green-500 text-white'
              : 'border-gray-300 dark:border-surface-600 hover:border-primary-500'
          )}
        >
          {task.status === 'completed' && <Check className="w-3 h-3" />}
        </button>

        <div className="flex-1 min-w-0">
          <p
            className={clsx(
              'text-sm',
              task.status === 'completed'
                ? 'text-gray-400 line-through'
                : 'text-gray-900 dark:text-white'
            )}
          >
            {task.text}
          </p>

          <div className="flex items-center gap-2 mt-1 flex-wrap">
            <Badge
              variant={
                task.priority === 'critical'
                  ? 'danger'
                  : task.priority === 'high'
                  ? 'danger'
                  : task.priority === 'medium'
                  ? 'warning'
                  : 'default'
              }
              size="sm"
            >
              {task.priority}
            </Badge>

            {task.assignee && (
              <span className="text-xs text-gray-500">{task.assignee}</span>
            )}

            {task.dueDate && (
              <span
                className={clsx(
                  'flex items-center gap-1 text-xs',
                  isOverdue ? 'text-red-500' : 'text-gray-500'
                )}
              >
                {isOverdue ? (
                  <AlertCircle className="w-3 h-3" />
                ) : (
                  <Clock className="w-3 h-3" />
                )}
                {formatDate(task.dueDate)}
              </span>
            )}
          </div>
        </div>

        <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setEditingTask(task)}
          >
            <Edit2 className="w-3 h-3" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => deleteTask(task.id)}
            className="text-red-500"
          >
            <Trash2 className="w-3 h-3" />
          </Button>
        </div>
      </div>
    );
  };

  const TaskGroup = ({
    title,
    tasks,
    icon,
    iconColor,
  }: {
    title: string;
    tasks: ActionItem[];
    icon?: React.ReactNode;
    iconColor?: string;
  }) => {
    if (tasks.length === 0) return null;

    return (
      <div className="mb-6">
        <div className="flex items-center gap-2 mb-3">
          {icon && <span className={iconColor}>{icon}</span>}
          <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300">
            {title}
          </h3>
          <Badge variant="default" size="sm">
            {tasks.length}
          </Badge>
        </div>
        <div className="space-y-2">
          {tasks.map((task) => (
            <TaskItem key={task.id} task={task} />
          ))}
        </div>
      </div>
    );
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Tasks
          </h1>
          <p className="text-gray-500 dark:text-gray-400 mt-1">
            {tasks.length} tasks total
          </p>
        </div>
        <Button
          variant="primary"
          onClick={() => setIsAddingTask(true)}
          icon={<Plus className="w-4 h-4" />}
        >
          Add Task
        </Button>
      </div>

      <Card padding="md" className="mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <Select
            value={filters.status}
            onChange={(value) => setFilter('status', value as TaskStatus | 'all')}
            options={[
              { value: 'all', label: 'All Status' },
              { value: 'pending', label: 'Pending' },
              { value: 'in_progress', label: 'In Progress' },
              { value: 'completed', label: 'Completed' },
            ]}
          />

          <Select
            value={filters.priority}
            onChange={(value) => setFilter('priority', value as Priority | 'all')}
            options={[
              { value: 'all', label: 'All Priority' },
              { value: 'critical', label: 'Critical' },
              { value: 'high', label: 'High' },
              { value: 'medium', label: 'Medium' },
              { value: 'low', label: 'Low' },
            ]}
          />

          {(filters.status !== 'all' || filters.priority !== 'all') && (
            <Button variant="ghost" size="sm" onClick={clearFilters}>
              <X className="w-4 h-4 mr-1" />
              Clear
            </Button>
          )}
        </div>
      </Card>

      <TaskGroup
        title="Overdue"
        tasks={groupedTasks.overdue}
        icon={<AlertCircle className="w-4 h-4" />}
        iconColor="text-red-500"
      />

      <TaskGroup
        title="Today"
        tasks={groupedTasks.today}
        icon={<Clock className="w-4 h-4" />}
        iconColor="text-amber-500"
      />

      <TaskGroup title="Upcoming" tasks={groupedTasks.upcoming} />

      <TaskGroup title="No Due Date" tasks={groupedTasks.noDue} />

      <TaskGroup
        title="Completed"
        tasks={groupedTasks.completed}
        icon={<Check className="w-4 h-4" />}
        iconColor="text-green-500"
      />

      {tasks.length === 0 && (
        <div className="text-center py-12">
          <Check className="w-12 h-12 mx-auto mb-4 text-gray-300" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
            No tasks
          </h3>
          <p className="text-gray-500">Add a task or analyze a meeting to get started</p>
        </div>
      )}

      <Modal
        isOpen={isAddingTask}
        onClose={() => setIsAddingTask(false)}
        title="Add New Task"
        size="md"
      >
        <div className="space-y-4">
          <Input
            label="Task"
            value={newTask.text}
            onChange={(e) => setNewTask({ ...newTask, text: e.target.value })}
            placeholder="What needs to be done?"
            autoFocus
          />

          <div className="grid grid-cols-2 gap-4">
            <Input
              label="Assignee"
              value={newTask.assignee}
              onChange={(e) => setNewTask({ ...newTask, assignee: e.target.value })}
              placeholder="Who is responsible?"
            />

            <Input
              label="Due Date"
              type="date"
              value={newTask.dueDate}
              onChange={(e) => setNewTask({ ...newTask, dueDate: e.target.value })}
            />
          </div>

          <Select
            label="Priority"
            value={newTask.priority}
            onChange={(value) => setNewTask({ ...newTask, priority: value as Priority })}
            options={[
              { value: 'low', label: 'Low' },
              { value: 'medium', label: 'Medium' },
              { value: 'high', label: 'High' },
              { value: 'critical', label: 'Critical' },
            ]}
          />

          <div className="flex justify-end gap-3 pt-4">
            <Button variant="secondary" onClick={() => setIsAddingTask(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={handleAddTask}>
              Add Task
            </Button>
          </div>
        </div>
      </Modal>

      <Modal
        isOpen={!!editingTask}
        onClose={() => setEditingTask(null)}
        title="Edit Task"
        size="md"
      >
        {editingTask && (
          <div className="space-y-4">
            <Input
              label="Task"
              value={editingTask.text}
              onChange={(e) =>
                setEditingTask({ ...editingTask, text: e.target.value })
              }
            />

            <div className="grid grid-cols-2 gap-4">
              <Input
                label="Assignee"
                value={editingTask.assignee || ''}
                onChange={(e) =>
                  setEditingTask({ ...editingTask, assignee: e.target.value })
                }
              />

              <Input
                label="Due Date"
                type="date"
                value={editingTask.dueDate?.split('T')[0] || ''}
                onChange={(e) =>
                  setEditingTask({ ...editingTask, dueDate: e.target.value })
                }
              />
            </div>

            <Select
              label="Priority"
              value={editingTask.priority}
              onChange={(value) =>
                setEditingTask({ ...editingTask, priority: value as Priority })
              }
              options={[
                { value: 'low', label: 'Low' },
                { value: 'medium', label: 'Medium' },
                { value: 'high', label: 'High' },
                { value: 'critical', label: 'Critical' },
              ]}
            />

            <div className="flex justify-end gap-3 pt-4">
              <Button variant="secondary" onClick={() => setEditingTask(null)}>
                Cancel
              </Button>
              <Button variant="primary" onClick={handleUpdateTask}>
                Save Changes
              </Button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
}
