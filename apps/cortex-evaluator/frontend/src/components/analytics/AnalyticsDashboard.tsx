import { useEffect } from 'react';
import { useEvaluationStats } from '../../hooks/useEvaluationHistory';
import type { ReactNode } from 'react';
import { apiClient } from '../../services/api';

interface AnalyticsDashboardProps {
  projectId?: string;
  dateFrom?: string;
  dateTo?: string;
}

export default function AnalyticsDashboard({ projectId, dateFrom, dateTo }: AnalyticsDashboardProps) {
  const { stats, loading, error, refetch } = useEvaluationStats(projectId, dateFrom, dateTo);

  useEffect(() => {
    refetch();
  }, [projectId, dateFrom, dateTo, refetch]);

  if (loading) {
    return (
      <div className="text-center py-12">
        <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
        <p className="mt-2 text-sm text-gray-600">Loading analytics...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-50 border border-red-200 rounded-lg p-4">
        <p className="text-red-600 text-sm">{error}</p>
      </div>
    );
  }

  if (!stats) {
    return null;
  }

  const providerEntries = Object.entries(stats.providerUsage);
  const typeEntries = Object.entries(stats.typeDistribution);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Analytics Dashboard</h2>
        <button
          onClick={() => refetch()}
          className="px-3 py-1.5 text-sm text-blue-600 hover:text-blue-700 transition-colors"
        >
          Refresh
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Total Evaluations" value={stats.totalEvaluations} color="blue" />
        <StatCard label="Avg Value Score" value={stats.avgValueScore.toFixed(1)} color="green" />
        <StatCard label="Median Score" value={stats.medianValueScore.toFixed(1)} color="purple" />
        <StatCard
          label="Implementation Rate"
          value={`${stats.implementationRate.rate.toFixed(1)}%`}
          color="orange"
          subtext={`${stats.implementationRate.implemented} of ${stats.implementationRate.total}`}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <ChartCard title="Provider Usage">
          <div className="space-y-3">
            {providerEntries.length === 0 ? (
              <p className="text-sm text-gray-500 text-center py-4">No data available</p>
            ) : (
              providerEntries.map(([provider, count]) => {
                const percentage = (count / stats.totalEvaluations) * 100;
                const providerColors: Record<string, string> = {
                  openai: 'bg-green-500',
                  anthropic: 'bg-blue-500',
                  google: 'bg-purple-500',
                  local: 'bg-orange-500',
                };
                const bgColor = providerColors[provider] || 'bg-gray-500';

                return (
                  <div key={provider}>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="capitalize text-gray-700">{provider}</span>
                      <span className="text-gray-600">{count} ({percentage.toFixed(1)}%)</span>
                    </div>
                    <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                      <div className={`h-full ${bgColor} transition-all duration-500`} style={{ width: `${percentage}%` }} />
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </ChartCard>

        <ChartCard title="Input Type Distribution">
          <div className="space-y-3">
            {typeEntries.length === 0 ? (
              <p className="text-sm text-gray-500 text-center py-4">No data available</p>
            ) : (
              typeEntries.map(([type, count]) => {
                const percentage = (count / stats.totalEvaluations) * 100;
                const typeColors: Record<string, string> = {
                  repo: 'bg-blue-500',
                  pdf: 'bg-red-500',
                  snippet: 'bg-green-500',
                  arxiv: 'bg-purple-500',
                  url: 'bg-yellow-500',
                };
                const bgColor = typeColors[type] || 'bg-gray-500';

                return (
                  <div key={type}>
                    <div className="flex justify-between text-sm mb-1">
                      <span className="capitalize text-gray-700">{type}</span>
                      <span className="text-gray-600">{count} ({percentage.toFixed(1)}%)</span>
                    </div>
                    <div className="h-2 bg-gray-200 rounded-full overflow-hidden">
                      <div className={`h-full ${bgColor} transition-all duration-500`} style={{ width: `${percentage}%` }} />
                    </div>
                  </div>
                );
              })
            )}
          </div>
        </ChartCard>
      </div>

      <ChartCard title="Implementation Status">
        <div className="grid grid-cols-3 gap-4">
          <StatusBlock
            label="Implemented"
            count={stats.implementationRate.implemented}
            total={stats.implementationRate.total}
            color="green"
          />
          <StatusBlock
            label="In Progress"
            count={stats.implementationRate.inProgress}
            total={stats.implementationRate.total}
            color="blue"
          />
          <StatusBlock
            label="Pending"
            count={stats.implementationRate.pending}
            total={stats.implementationRate.total}
            color="gray"
          />
        </div>
      </ChartCard>

      <ChartCard title="Activity Trend">
        <TrendChart stats={stats} />
      </ChartCard>
    </div>
  );
}

function StatCard({ label, value, color, subtext }: { label: string; value: string | number; color: string; subtext?: string }) {
  const colorClasses: Record<string, string> = {
    blue: 'bg-blue-50 border-blue-200 text-blue-700',
    green: 'bg-green-50 border-green-200 text-green-700',
    purple: 'bg-purple-50 border-purple-200 text-purple-700',
    orange: 'bg-orange-50 border-orange-200 text-orange-700',
  };
  const colorClass = colorClasses[color] || colorClasses.blue;

  return (
    <div className={`p-4 border-2 rounded-lg ${colorClass}`}>
      <p className="text-xs uppercase tracking-wide font-medium opacity-80">{label}</p>
      <p className="text-2xl font-bold mt-1">{value}</p>
      {subtext && <p className="text-xs opacity-70 mt-1">{subtext}</p>}
    </div>
  );
}

function ChartCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="bg-white border-2 border-gray-200 rounded-lg p-4">
      <h3 className="text-sm font-semibold text-gray-900 mb-4">{title}</h3>
      {children}
    </div>
  );
}

function StatusBlock({ label, count, total, color }: { label: string; count: number; total: number; color: string }) {
  const percentage = total > 0 ? ((count / total) * 100).toFixed(0) : 0;
  const colorClasses: Record<string, string> = {
    green: 'bg-green-100 text-green-700 border-green-200',
    blue: 'bg-blue-100 text-blue-700 border-blue-200',
    gray: 'bg-gray-100 text-gray-700 border-gray-200',
  };
  const colorClass = colorClasses[color] || colorClasses.gray;

  return (
    <div className={`p-4 border-2 rounded-lg ${colorClass}`}>
      <p className="text-xs uppercase tracking-wide font-medium opacity-80 mb-1">{label}</p>
      <p className="text-xl font-bold">{count}</p>
      <p className="text-xs opacity-70">{percentage}%</p>
    </div>
  );
}

function TrendChart({ stats }: { stats: NonNullable<ReturnType<typeof useEvaluationStats>['stats']> }) {
  const trendData = [
    { label: '7 Days', count: stats.trend.last7Days },
    { label: '30 Days', count: stats.trend.last30Days },
    { label: '90 Days', count: stats.trend.last90Days },
  ];

  const maxCount = Math.max(...trendData.map((d) => d.count));

  return (
    <div className="space-y-4">
      {trendData.map((data, index) => {
        const percentage = maxCount > 0 ? (data.count / maxCount) * 100 : 0;
        const colors = ['bg-blue-500', 'bg-green-500', 'bg-purple-500'];
        return (
          <div key={data.label}>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-700">{data.label}</span>
              <span className="text-gray-600 font-medium">{data.count} evaluations</span>
            </div>
            <div className="h-6 bg-gray-100 rounded-lg overflow-hidden flex items-center px-2">
              <div
                className={`h-full ${colors[index]} rounded-md transition-all duration-500`}
                style={{ width: `${percentage}%` }}
              />
            </div>
          </div>
        );
      })}
    </div>
  );
}
