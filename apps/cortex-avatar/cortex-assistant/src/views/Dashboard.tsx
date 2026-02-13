import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Plus,
  Clock,
  FileText,
  Sparkles,
  ListTodo,
  TrendingUp,
  ArrowRight,
} from 'lucide-react';
import { useMeetingStore, useTasksStore } from '@/store';
import { getRecentMeetings, getMeetingStats } from '@/services/meeting';
import { formatRelativeTime, formatDuration } from '@/utils/format';
import { Button, Card, CardTitle, Badge } from '@/components/ui';
import type { MeetingSession } from '@/models';

export function Dashboard() {
  const navigate = useNavigate();
  const { createNewMeeting, loadMeeting } = useMeetingStore();
  const { getTaskStats } = useTasksStore();
  const [recentMeetings, setRecentMeetings] = useState<MeetingSession[]>([]);
  const [stats, setStats] = useState({
    total: 0,
    analyzed: 0,
    totalDuration: 0,
    averageDuration: 0,
    meetingsThisWeek: 0,
    meetingsThisMonth: 0,
  });

  const taskStats = getTaskStats();

  useEffect(() => {
    const load = async () => {
      const meetings = await getRecentMeetings(5);
      setRecentMeetings(meetings);
      const meetingStats = await getMeetingStats();
      setStats(meetingStats);
    };
    load();
  }, []);

  const handleNewMeeting = () => {
    createNewMeeting();
    navigate('/meeting');
  };

  const handleOpenMeeting = (meeting: MeetingSession) => {
    loadMeeting(meeting);
    navigate('/meeting');
  };

  const statCards = [
    {
      label: 'Total Meetings',
      value: stats.total,
      icon: FileText,
      color: 'text-blue-500',
      bg: 'bg-blue-50 dark:bg-blue-900/20',
    },
    {
      label: 'Analyzed',
      value: stats.analyzed,
      icon: Sparkles,
      color: 'text-purple-500',
      bg: 'bg-purple-50 dark:bg-purple-900/20',
    },
    {
      label: 'This Week',
      value: stats.meetingsThisWeek,
      icon: TrendingUp,
      color: 'text-green-500',
      bg: 'bg-green-50 dark:bg-green-900/20',
    },
    {
      label: 'Pending Tasks',
      value: taskStats.pending + taskStats.inProgress,
      icon: ListTodo,
      color: 'text-amber-500',
      bg: 'bg-amber-50 dark:bg-amber-900/20',
    },
  ];

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Welcome to Cortex Assistant
          </h1>
          <p className="text-gray-500 dark:text-gray-400 mt-1">
            Your AI-powered meeting assistant
          </p>
        </div>
        <Button
          variant="primary"
          onClick={handleNewMeeting}
          icon={<Plus className="w-4 h-4" />}
        >
          New Meeting
        </Button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {statCards.map((stat) => (
          <Card key={stat.label} padding="md">
            <div className="flex items-center gap-4">
              <div className={`p-3 rounded-lg ${stat.bg}`}>
                <stat.icon className={`w-6 h-6 ${stat.color}`} />
              </div>
              <div>
                <p className="text-2xl font-bold text-gray-900 dark:text-white">
                  {stat.value}
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  {stat.label}
                </p>
              </div>
            </div>
          </Card>
        ))}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <Card padding="lg">
          <div className="flex items-center justify-between mb-4">
            <CardTitle>Recent Meetings</CardTitle>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => navigate('/history')}
              icon={<ArrowRight className="w-4 h-4" />}
              iconPosition="right"
            >
              View All
            </Button>
          </div>

          {recentMeetings.length === 0 ? (
            <div className="text-center py-8 text-gray-500">
              <FileText className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>No meetings yet</p>
              <p className="text-sm">Start your first meeting to see it here</p>
            </div>
          ) : (
            <div className="space-y-3">
              {recentMeetings.map((meeting) => (
                <div
                  key={meeting.id}
                  className="flex items-center justify-between p-3 rounded-lg hover:bg-gray-50 dark:hover:bg-surface-800 cursor-pointer transition-colors"
                  onClick={() => handleOpenMeeting(meeting)}
                >
                  <div className="min-w-0">
                    <h3 className="font-medium text-gray-900 dark:text-white truncate">
                      {meeting.title}
                    </h3>
                    <div className="flex items-center gap-2 text-sm text-gray-500">
                      <Clock className="w-3 h-3" />
                      <span>{formatRelativeTime(meeting.createdAt)}</span>
                      <span>•</span>
                      <span>{formatDuration(meeting.duration)}</span>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    {meeting.isAnalyzed && (
                      <Badge variant="success" size="sm">
                        Analyzed
                      </Badge>
                    )}
                    <ArrowRight className="w-4 h-4 text-gray-400" />
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card padding="lg">
          <div className="flex items-center justify-between mb-4">
            <CardTitle>Pending Tasks</CardTitle>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => navigate('/tasks')}
              icon={<ArrowRight className="w-4 h-4" />}
              iconPosition="right"
            >
              View All
            </Button>
          </div>

          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <span className="text-gray-600 dark:text-gray-400">Overdue</span>
              <Badge variant={taskStats.overdue > 0 ? 'danger' : 'default'}>
                {taskStats.overdue}
              </Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-gray-600 dark:text-gray-400">In Progress</span>
              <Badge variant="primary">{taskStats.inProgress}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-gray-600 dark:text-gray-400">Pending</span>
              <Badge variant="warning">{taskStats.pending}</Badge>
            </div>
            <div className="flex items-center justify-between">
              <span className="text-gray-600 dark:text-gray-400">Completed</span>
              <Badge variant="success">{taskStats.completed}</Badge>
            </div>
          </div>

          <div className="mt-6 pt-4 border-t border-gray-200 dark:border-surface-700">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-500">Total meeting time</span>
              <span className="font-medium text-gray-900 dark:text-white">
                {formatDuration(stats.totalDuration)}
              </span>
            </div>
            <div className="flex items-center justify-between text-sm mt-2">
              <span className="text-gray-500">Average meeting</span>
              <span className="font-medium text-gray-900 dark:text-white">
                {formatDuration(stats.averageDuration)}
              </span>
            </div>
          </div>
        </Card>
      </div>

      <div className="mt-8 p-6 bg-gradient-to-r from-primary-500 to-primary-600 rounded-xl text-white">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-xl font-bold mb-2">Quick Start</h2>
            <p className="text-primary-100">
              Press <kbd className="px-2 py-1 bg-white/20 rounded">Ctrl+K</kbd> to open the command palette
            </p>
          </div>
          <Button
            variant="secondary"
            onClick={handleNewMeeting}
            className="bg-white text-primary-600 hover:bg-primary-50"
          >
            Start Meeting
          </Button>
        </div>
      </div>
    </div>
  );
}
