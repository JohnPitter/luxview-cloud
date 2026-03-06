import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
} from 'recharts';
import { GlassCard } from '../common/GlassCard';
import { useThemeStore } from '../../stores/theme.store';

interface MetricsChartProps {
  data: Array<Record<string, unknown>>;
  dataKey: string;
  title: string;
  color: string;
  formatter?: (value: number) => string;
  unit?: string;
}

export function MetricsChart({ data, dataKey, title, color, formatter, unit }: MetricsChartProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const formatValue = (value: number) => {
    if (formatter) return formatter(value);
    return `${value}${unit || ''}`;
  };

  return (
    <GlassCard>
      <h3
        className={`text-sm font-semibold mb-4 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
      >
        {title}
      </h3>
      <ResponsiveContainer width="100%" height={200}>
        <AreaChart data={data}>
          <defs>
            <linearGradient id={`gradient-${dataKey}`} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.3} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid
            strokeDasharray="3 3"
            stroke={isDark ? '#27272a' : '#e4e4e7'}
            vertical={false}
          />
          <XAxis
            dataKey="time"
            tick={{ fontSize: 11, fill: isDark ? '#71717a' : '#a1a1aa' }}
            axisLine={false}
            tickLine={false}
          />
          <YAxis
            tick={{ fontSize: 11, fill: isDark ? '#71717a' : '#a1a1aa' }}
            axisLine={false}
            tickLine={false}
            tickFormatter={(v) => formatValue(v as number)}
            width={50}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: isDark ? '#18181b' : '#ffffff',
              border: `1px solid ${isDark ? '#3f3f46' : '#e4e4e7'}`,
              borderRadius: '12px',
              boxShadow: '0 4px 6px -1px rgba(0,0,0,0.3)',
              fontSize: '12px',
            }}
            labelStyle={{ color: isDark ? '#a1a1aa' : '#71717a' }}
            formatter={(value: number) => [formatValue(value), title]}
          />
          <Area
            type="monotone"
            dataKey={dataKey}
            stroke={color}
            strokeWidth={2}
            fill={`url(#gradient-${dataKey})`}
            animationDuration={300}
          />
        </AreaChart>
      </ResponsiveContainer>
    </GlassCard>
  );
}
