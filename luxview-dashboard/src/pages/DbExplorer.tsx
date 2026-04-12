import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft,
  Table2,
  Play,
  Loader2,
  ChevronRight,
  Columns3,
  Hash,
  AlertCircle,
  Copy,
  Check,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import {
  servicesApi,
  type TableInfo,
  type TableSchema,
  type QueryResult,
} from '../api/services';

export function DbExplorer() {
  const { serviceId } = useParams<{ serviceId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const editorRef = useRef<HTMLTextAreaElement>(null);

  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTable, setSelectedTable] = useState<string | null>(null);
  const [schema, setSchema] = useState<TableSchema | null>(null);
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState({ tables: true, schema: false, query: false });
  const [copied, setCopied] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'results' | 'schema'>('results');

  const fetchTables = useCallback(async () => {
    if (!serviceId) return;
    setLoading((l) => ({ ...l, tables: true }));
    try {
      const data = await servicesApi.listTables(serviceId);
      setTables(data);
    } catch {
      setError(t('resources.db.failedToLoadTables'));
    } finally {
      setLoading((l) => ({ ...l, tables: false }));
    }
  }, [serviceId, t]);

  useEffect(() => {
    fetchTables();
  }, [fetchTables]);

  const selectTable = async (tableName: string) => {
    if (!serviceId) return;
    setSelectedTable(tableName);
    setActiveTab('schema');
    setLoading((l) => ({ ...l, schema: true }));
    try {
      const data = await servicesApi.getTableSchema(serviceId, tableName);
      setSchema(data);
      setQuery(`SELECT * FROM "${tableName}" LIMIT 100;`);
    } catch {
      setError(t('resources.db.failedToLoadSchema'));
    } finally {
      setLoading((l) => ({ ...l, schema: false }));
    }
  };

  const executeQuery = async () => {
    if (!serviceId || !query.trim()) return;
    setError(null);
    setResult(null);
    setActiveTab('results');
    setLoading((l) => ({ ...l, query: true }));
    try {
      const data = await servicesApi.executeQuery(serviceId, query);
      setResult(data);
    } catch (err: unknown) {
      const msg =
        err && typeof err === 'object' && 'response' in err
          ? (err as { response: { data: { error: string } } }).response?.data?.error
          : t('resources.db.queryFailed');
      setError(msg || t('resources.db.queryFailed'));
    } finally {
      setLoading((l) => ({ ...l, query: false }));
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      executeQuery();
    }
  };

  const copyValue = (val: string, key: string) => {
    navigator.clipboard.writeText(val);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  };

  const formatValue = (val: unknown): string => {
    if (val === null || val === undefined) return 'NULL';
    if (typeof val === 'object') return JSON.stringify(val);
    return String(val);
  };

  const cellBg = isDark ? 'bg-zinc-900/40' : 'bg-zinc-50';
  const headerBg = isDark ? 'bg-zinc-800/60' : 'bg-zinc-100';
  const borderColor = isDark ? 'border-zinc-800/50' : 'border-zinc-200/60';

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center gap-3 mb-6">
        <button
          onClick={() => navigate('/dashboard/resources')}
          className={`p-2 rounded-xl transition-colors ${
            isDark ? 'hover:bg-zinc-800' : 'hover:bg-zinc-100'
          }`}
        >
          <ArrowLeft size={18} className={isDark ? 'text-zinc-400' : 'text-zinc-600'} />
        </button>
        <div>
          <h1
            className={`text-xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            {t('resources.db.title')}
          </h1>
          <p className="text-xs text-zinc-500">
            {t('resources.db.subtitle')}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-12 gap-4" style={{ minHeight: 'calc(100vh - 200px)' }}>
        {/* Sidebar: Table list */}
        <div className="col-span-3">
          <GlassCard className="!p-0 overflow-hidden">
            <div
              className={`px-4 py-3 border-b ${borderColor} flex items-center justify-between`}
            >
              <span
                className={`text-xs font-semibold uppercase tracking-wider ${
                  isDark ? 'text-zinc-400' : 'text-zinc-600'
                }`}
              >
                {t('resources.db.tables')}
              </span>
              <span
                className={`text-[10px] font-medium px-2 py-0.5 rounded-full ${
                  isDark ? 'bg-zinc-800 text-zinc-400' : 'bg-zinc-200 text-zinc-500'
                }`}
              >
                {tables.length}
              </span>
            </div>
            <div className="max-h-[600px] overflow-y-auto">
              {loading.tables ? (
                <div className="p-4 space-y-2">
                  {[1, 2, 3, 4].map((i) => (
                    <div
                      key={i}
                      className={`h-8 rounded-lg animate-pulse ${
                        isDark ? 'bg-zinc-800/50' : 'bg-zinc-100'
                      }`}
                    />
                  ))}
                </div>
              ) : tables.length === 0 ? (
                <div className="p-6 text-center">
                  <Table2 size={24} className="mx-auto text-zinc-500 mb-2" />
                  <p className="text-xs text-zinc-500">{t('resources.db.noTablesFound')}</p>
                </div>
              ) : (
                <div className="p-2">
                  {tables.map((tbl) => (
                    <button
                      key={tbl.name}
                      onClick={() => selectTable(tbl.name)}
                      className={`
                        w-full flex items-center gap-2 px-3 py-2 rounded-lg text-left text-sm
                        transition-all duration-150
                        ${
                          selectedTable === tbl.name
                            ? isDark
                              ? 'bg-amber-400/10 text-amber-400'
                              : 'bg-amber-100 text-amber-700'
                            : isDark
                              ? 'text-zinc-300 hover:bg-zinc-800/50'
                              : 'text-zinc-700 hover:bg-zinc-100'
                        }
                      `}
                    >
                      <Table2 size={14} className="flex-shrink-0 opacity-60" />
                      <span className="truncate font-mono text-xs">{tbl.name}</span>
                      <ChevronRight
                        size={12}
                        className={`ml-auto flex-shrink-0 opacity-40 ${
                          selectedTable === tbl.name ? 'opacity-100' : ''
                        }`}
                      />
                    </button>
                  ))}
                </div>
              )}
            </div>
          </GlassCard>
        </div>

        {/* Main area */}
        <div className="col-span-9 flex flex-col gap-4">
          {/* SQL Editor */}
          <GlassCard className="!p-0 overflow-hidden">
            <div
              className={`px-4 py-3 border-b ${borderColor} flex items-center justify-between`}
            >
              <span
                className={`text-xs font-semibold uppercase tracking-wider ${
                  isDark ? 'text-zinc-400' : 'text-zinc-600'
                }`}
              >
                {t('resources.db.sqlQuery')}
              </span>
              <div className="flex items-center gap-2">
                <span className="text-[10px] text-zinc-500">{t('resources.db.ctrlEnterToRun')}</span>
                <PillButton
                  variant="primary"
                  size="sm"
                  disabled={loading.query || !query.trim()}
                  onClick={executeQuery}
                  icon={
                    loading.query ? (
                      <Loader2 size={12} className="animate-spin" />
                    ) : (
                      <Play size={12} />
                    )
                  }
                >
                  {loading.query ? t('resources.db.running') : t('resources.db.execute')}
                </PillButton>
              </div>
            </div>
            <textarea
              ref={editorRef}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t('resources.db.queryPlaceholder')}
              spellCheck={false}
              className={`
                w-full p-4 font-mono text-sm resize-vertical outline-none
                ${isDark ? 'bg-zinc-950/50 text-zinc-200 placeholder:text-zinc-600' : 'bg-white text-zinc-800 placeholder:text-zinc-400'}
              `}
              style={{ minHeight: 100, maxHeight: 400 }}
              rows={6}
            />
          </GlassCard>

          {/* Results / Schema tabs */}
          <GlassCard className="!p-0 overflow-hidden flex-1">
            <div className={`flex border-b ${borderColor}`}>
              {(['results', 'schema'] as const).map((tab) => (
                <button
                  key={tab}
                  onClick={() => setActiveTab(tab)}
                  className={`
                    px-4 py-3 text-xs font-semibold uppercase tracking-wider transition-colors
                    ${
                      activeTab === tab
                        ? isDark
                          ? 'text-amber-400 border-b-2 border-amber-400'
                          : 'text-amber-600 border-b-2 border-amber-500'
                        : isDark
                          ? 'text-zinc-500 hover:text-zinc-300'
                          : 'text-zinc-400 hover:text-zinc-600'
                    }
                  `}
                >
                  {tab === 'results' ? t('resources.db.results') : t('resources.db.schema')}
                  {tab === 'results' && result && (
                    <span className="ml-2 text-[10px] opacity-70">
                      ({t('resources.db.rowCount', { count: result.rowCount })})
                    </span>
                  )}
                  {tab === 'schema' && schema && (
                    <span className="ml-2 text-[10px] opacity-70">
                      ({t('resources.db.colCount', { count: schema.columns.length })})
                    </span>
                  )}
                </button>
              ))}
            </div>

            <div className="overflow-auto" style={{ maxHeight: '500px' }}>
              {/* Error */}
              {error && (
                <div className="p-4 flex items-start gap-3">
                  <AlertCircle size={16} className="text-red-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className="text-sm text-red-400 font-medium">{t('resources.db.error')}</p>
                    <p className={`text-xs mt-1 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                      {error}
                    </p>
                  </div>
                </div>
              )}

              {/* Results tab */}
              {activeTab === 'results' && !error && (
                <>
                  {!result && !loading.query && (
                    <div className="p-12 text-center">
                      <Play size={32} className="mx-auto text-zinc-600 mb-3" />
                      <p className={`text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
                        {t('resources.db.runQueryToSeeResults')}
                      </p>
                    </div>
                  )}
                  {loading.query && (
                    <div className="p-12 text-center">
                      <Loader2 size={24} className="mx-auto text-amber-400 animate-spin mb-3" />
                      <p className="text-xs text-zinc-500">{t('resources.db.executingQuery')}</p>
                    </div>
                  )}
                  {result && result.columns.length > 0 && (
                    <>
                      {result.truncated && (
                        <div
                          className={`px-4 py-2 text-[11px] border-b ${borderColor} ${
                            isDark ? 'bg-amber-400/5 text-amber-400' : 'bg-amber-50 text-amber-600'
                          }`}
                        >
                          {t('resources.db.resultsTruncated')}
                        </div>
                      )}
                      <table className="w-full text-xs">
                        <thead>
                          <tr className={headerBg}>
                            <th
                              className={`px-3 py-2 text-left font-semibold border-b ${borderColor} ${
                                isDark ? 'text-zinc-400' : 'text-zinc-500'
                              }`}
                              style={{ width: 40 }}
                            >
                              #
                            </th>
                            {result.columns.map((col) => (
                              <th
                                key={col}
                                className={`px-3 py-2 text-left font-semibold border-b ${borderColor} ${
                                  isDark ? 'text-zinc-400' : 'text-zinc-500'
                                }`}
                              >
                                <span className="font-mono">{col}</span>
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {result.rows.map((row, i) => (
                            <tr
                              key={i}
                              className={`${
                                isDark ? 'hover:bg-zinc-800/30' : 'hover:bg-zinc-50'
                              } transition-colors`}
                            >
                              <td
                                className={`px-3 py-1.5 border-b ${borderColor} text-zinc-500 font-mono`}
                              >
                                {i + 1}
                              </td>
                              {result.columns.map((col) => {
                                const val = formatValue(row[col]);
                                const isNull = row[col] === null || row[col] === undefined;
                                const cellKey = `${i}-${col}`;
                                return (
                                  <td
                                    key={col}
                                    className={`px-3 py-1.5 border-b ${borderColor} font-mono group relative`}
                                  >
                                    <div className="flex items-center gap-1 max-w-[300px]">
                                      <span
                                        className={`truncate ${
                                          isNull
                                            ? 'text-zinc-500 italic'
                                            : isDark
                                              ? 'text-zinc-300'
                                              : 'text-zinc-700'
                                        }`}
                                        title={val}
                                      >
                                        {val}
                                      </span>
                                      <button
                                        onClick={() => copyValue(val, cellKey)}
                                        className="opacity-0 group-hover:opacity-100 transition-opacity text-zinc-500 hover:text-amber-400 flex-shrink-0"
                                      >
                                        {copied === cellKey ? (
                                          <Check size={10} className="text-emerald-400" />
                                        ) : (
                                          <Copy size={10} />
                                        )}
                                      </button>
                                    </div>
                                  </td>
                                );
                              })}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </>
                  )}
                  {result && result.columns.length === 0 && (
                    <div className="p-8 text-center">
                      <Check size={24} className="mx-auto text-emerald-400 mb-2" />
                      <p className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                        {t('resources.db.queryExecutedSuccessfully')}
                      </p>
                      <p className="text-xs text-zinc-500 mt-1">{t('resources.db.noRowsReturned')}</p>
                    </div>
                  )}
                </>
              )}

              {/* Schema tab */}
              {activeTab === 'schema' && (
                <>
                  {!selectedTable && (
                    <div className="p-12 text-center">
                      <Columns3 size={32} className="mx-auto text-zinc-600 mb-3" />
                      <p className={`text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
                        {t('resources.db.selectTableToViewSchema')}
                      </p>
                    </div>
                  )}
                  {loading.schema && (
                    <div className="p-12 text-center">
                      <Loader2 size={24} className="mx-auto text-amber-400 animate-spin mb-3" />
                      <p className="text-xs text-zinc-500">{t('resources.db.loadingSchema')}</p>
                    </div>
                  )}
                  {schema && !loading.schema && (
                    <>
                      <div
                        className={`px-4 py-2 border-b ${borderColor} flex items-center gap-3`}
                      >
                        <span className={`text-sm font-mono font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                          {selectedTable}
                        </span>
                        <span className="flex items-center gap-1 text-[10px] text-zinc-500">
                          <Hash size={10} />
                          {t('resources.db.rowCount', { count: schema.rowCount })}
                        </span>
                      </div>
                      <table className="w-full text-xs">
                        <thead>
                          <tr className={headerBg}>
                            {[
                              t('resources.db.schemaHeaders.column'),
                              t('resources.db.schemaHeaders.type'),
                              t('resources.db.schemaHeaders.nullable'),
                              t('resources.db.schemaHeaders.default'),
                            ].map((h) => (
                              <th
                                key={h}
                                className={`px-3 py-2 text-left font-semibold border-b ${borderColor} ${
                                  isDark ? 'text-zinc-400' : 'text-zinc-500'
                                }`}
                              >
                                {h}
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {schema.columns.map((col) => (
                            <tr
                              key={col.name}
                              className={`${
                                isDark ? 'hover:bg-zinc-800/30' : 'hover:bg-zinc-50'
                              } transition-colors`}
                            >
                              <td
                                className={`px-3 py-2 border-b ${borderColor} font-mono font-medium ${
                                  isDark ? 'text-zinc-200' : 'text-zinc-800'
                                }`}
                              >
                                {col.name}
                              </td>
                              <td
                                className={`px-3 py-2 border-b ${borderColor}`}
                              >
                                <span
                                  className={`px-2 py-0.5 rounded-md text-[10px] font-medium ${
                                    isDark
                                      ? 'bg-blue-500/10 text-blue-400'
                                      : 'bg-blue-100 text-blue-700'
                                  }`}
                                >
                                  {col.type}
                                </span>
                              </td>
                              <td
                                className={`px-3 py-2 border-b ${borderColor} ${
                                  isDark ? 'text-zinc-400' : 'text-zinc-600'
                                }`}
                              >
                                {col.nullable === 'YES' ? (
                                  <span className="text-amber-400">{t('resources.db.nullable')}</span>
                                ) : (
                                  <span className="text-emerald-400">{t('resources.db.notNull')}</span>
                                )}
                              </td>
                              <td
                                className={`px-3 py-2 border-b ${borderColor} font-mono ${
                                  isDark ? 'text-zinc-500' : 'text-zinc-400'
                                }`}
                              >
                                {col.default || '-'}
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </>
                  )}
                </>
              )}
            </div>
          </GlassCard>
        </div>
      </div>
    </div>
  );
}
