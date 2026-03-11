import { useState, useEffect, useCallback, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft,
  Upload,
  Download,
  Trash2,
  Folder,
  FileIcon,
  Loader2,
  ChevronRight,
  RefreshCw,
  AlertCircle,
  Search,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { ConfirmDialog } from '../components/common/ConfirmDialog';
import { useThemeStore } from '../stores/theme.store';
import { servicesApi, type StorageFileInfo } from '../api/services';

function formatSize(bytes: number): string {
  if (bytes === 0) return '-';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

export function StorageExplorer() {
  const { serviceId } = useParams<{ serviceId: string }>();
  const navigate = useNavigate();
  const { t, i18n } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [files, setFiles] = useState<StorageFileInfo[]>([]);
  const [prefix, setPrefix] = useState('');
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [downloadingKey, setDownloadingKey] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState('');

  const formatDate = (dateStr: string): string => {
    if (!dateStr) return '-';
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return '-';
    return d.toLocaleDateString(i18n.language, {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const fetchFiles = useCallback(async () => {
    if (!serviceId) return;
    setLoading(true);
    setError(null);
    try {
      const data = await servicesApi.listFiles(serviceId, prefix || undefined);
      setFiles(data);
    } catch {
      setError(t('resources.storage.failedToLoadFiles'));
    } finally {
      setLoading(false);
    }
  }, [serviceId, prefix, t]);

  useEffect(() => {
    fetchFiles();
  }, [fetchFiles]);

  const navigateToFolder = (key: string) => {
    setPrefix(key);
    setSearchFilter('');
  };

  const navigateUp = () => {
    const parts = prefix.split('/').filter(Boolean);
    parts.pop();
    setPrefix(parts.length > 0 ? parts.join('/') + '/' : '');
  };

  const breadcrumbs = prefix
    .split('/')
    .filter(Boolean)
    .map((part, i, arr) => ({
      label: part,
      path: arr.slice(0, i + 1).join('/') + '/',
    }));

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const fileList = e.target.files;
    if (!fileList || !serviceId) return;
    setUploading(true);
    setError(null);
    try {
      for (const file of Array.from(fileList)) {
        const key = prefix + file.name;
        await servicesApi.uploadFile(serviceId, file, key);
      }
      await fetchFiles();
    } catch {
      setError(t('resources.storage.failedToUploadFile'));
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = '';
    }
  };

  const handleDownload = async (key: string) => {
    if (!serviceId) return;
    setDownloadingKey(key);
    try {
      const blob = await servicesApi.downloadFile(serviceId, key);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = key.split('/').pop() || 'download';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
    } catch {
      setError(t('resources.storage.failedToDownloadFile'));
    } finally {
      setDownloadingKey(null);
    }
  };

  const handleDelete = async () => {
    if (!serviceId || !deleteTarget) return;
    setDeleting(true);
    try {
      await servicesApi.deleteFile(serviceId, deleteTarget);
      setDeleteTarget(null);
      await fetchFiles();
    } catch {
      setError(t('resources.storage.failedToDeleteFile'));
    } finally {
      setDeleting(false);
    }
  };

  const filteredFiles = searchFilter
    ? files.filter((f) => f.key.toLowerCase().includes(searchFilter.toLowerCase()))
    : files;

  const borderColor = isDark ? 'border-zinc-800/50' : 'border-zinc-200/60';

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
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
              {t('resources.storage.title')}
            </h1>
            <p className="text-xs text-zinc-500">{t('resources.storage.subtitle')}</p>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <PillButton variant="ghost" size="sm" onClick={fetchFiles} icon={<RefreshCw size={12} />}>
            {t('common.refresh')}
          </PillButton>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            onChange={handleUpload}
          />
          <PillButton
            variant="primary"
            size="sm"
            disabled={uploading}
            onClick={() => fileInputRef.current?.click()}
            icon={
              uploading ? (
                <Loader2 size={12} className="animate-spin" />
              ) : (
                <Upload size={12} />
              )
            }
          >
            {uploading ? t('resources.storage.uploading') : t('resources.storage.upload')}
          </PillButton>
        </div>
      </div>

      {/* Error */}
      {error && (
        <div
          className={`mb-4 px-4 py-3 rounded-xl flex items-center gap-3 ${
            isDark ? 'bg-red-500/10 border border-red-500/20' : 'bg-red-50 border border-red-200'
          }`}
        >
          <AlertCircle size={16} className="text-red-400 flex-shrink-0" />
          <p className="text-sm text-red-400">{error}</p>
        </div>
      )}

      <GlassCard className="!p-0 overflow-hidden">
        {/* Breadcrumbs + Search */}
        <div className={`px-4 py-3 border-b ${borderColor} flex items-center gap-3`}>
          <div className="flex items-center gap-1 flex-1 min-w-0">
            <button
              onClick={() => {
                setPrefix('');
                setSearchFilter('');
              }}
              className={`text-xs font-medium px-2 py-1 rounded transition-colors ${
                isDark
                  ? 'text-zinc-300 hover:bg-zinc-800'
                  : 'text-zinc-700 hover:bg-zinc-100'
              }`}
            >
              /
            </button>
            {breadcrumbs.map((bc) => (
              <div key={bc.path} className="flex items-center gap-1">
                <ChevronRight size={12} className="text-zinc-500" />
                <button
                  onClick={() => navigateToFolder(bc.path)}
                  className={`text-xs font-medium px-2 py-1 rounded truncate max-w-[120px] transition-colors ${
                    isDark
                      ? 'text-zinc-300 hover:bg-zinc-800'
                      : 'text-zinc-700 hover:bg-zinc-100'
                  }`}
                >
                  {bc.label}
                </button>
              </div>
            ))}
          </div>
          <div className="relative">
            <Search
              size={13}
              className="absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500"
            />
            <input
              type="text"
              placeholder={t('resources.storage.filterPlaceholder')}
              value={searchFilter}
              onChange={(e) => setSearchFilter(e.target.value)}
              className={`pl-7 pr-3 py-1.5 rounded-lg text-xs outline-none w-40 ${
                isDark
                  ? 'bg-zinc-800/60 text-zinc-300 border border-zinc-700/50 placeholder:text-zinc-600'
                  : 'bg-zinc-50 text-zinc-700 border border-zinc-200 placeholder:text-zinc-400'
              }`}
            />
          </div>
        </div>

        {/* File list */}
        {loading ? (
          <div className="p-8 text-center">
            <Loader2 size={24} className="mx-auto text-amber-400 animate-spin mb-3" />
            <p className="text-xs text-zinc-500">{t('resources.storage.loadingFiles')}</p>
          </div>
        ) : filteredFiles.length === 0 ? (
          <div className="p-12 text-center">
            <FileIcon size={32} className="mx-auto text-zinc-600 mb-3" />
            <p className={`text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
              {searchFilter ? t('resources.storage.noFilesMatchFilter') : t('resources.storage.folderEmpty')}
            </p>
            <p className="text-xs text-zinc-500 mt-1">
              {!searchFilter && t('resources.storage.uploadFilesHint')}
            </p>
          </div>
        ) : (
          <div>
            {/* Back to parent */}
            {prefix && (
              <button
                onClick={navigateUp}
                className={`w-full flex items-center gap-3 px-4 py-3 border-b ${borderColor} transition-colors text-left ${
                  isDark ? 'hover:bg-zinc-800/30' : 'hover:bg-zinc-50'
                }`}
              >
                <Folder size={16} className="text-amber-400" />
                <span className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                  ..
                </span>
              </button>
            )}
            {filteredFiles.map((file) => {
              const name = file.key.replace(prefix, '');
              if (!name) return null;
              return (
                <div
                  key={file.key}
                  className={`flex items-center gap-3 px-4 py-3 border-b last:border-b-0 ${borderColor} transition-colors group ${
                    isDark ? 'hover:bg-zinc-800/30' : 'hover:bg-zinc-50'
                  }`}
                >
                  {file.isDir ? (
                    <button
                      onClick={() => navigateToFolder(file.key)}
                      className="flex items-center gap-3 flex-1 min-w-0 text-left"
                    >
                      <Folder size={16} className="text-amber-400 flex-shrink-0" />
                      <span
                        className={`text-sm font-medium truncate ${
                          isDark ? 'text-zinc-200' : 'text-zinc-800'
                        }`}
                      >
                        {name.replace(/\/$/, '')}
                      </span>
                    </button>
                  ) : (
                    <div className="flex items-center gap-3 flex-1 min-w-0">
                      <FileIcon
                        size={16}
                        className={`flex-shrink-0 ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}
                      />
                      <span
                        className={`text-sm truncate ${
                          isDark ? 'text-zinc-300' : 'text-zinc-700'
                        }`}
                      >
                        {name}
                      </span>
                    </div>
                  )}
                  <span className="text-xs text-zinc-500 flex-shrink-0 w-20 text-right">
                    {formatSize(file.size)}
                  </span>
                  <span className="text-xs text-zinc-500 flex-shrink-0 w-36 text-right hidden md:block">
                    {formatDate(file.lastModified)}
                  </span>
                  {!file.isDir && (
                    <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0">
                      <button
                        onClick={() => handleDownload(file.key)}
                        disabled={downloadingKey === file.key}
                        className={`p-1.5 rounded-lg transition-colors ${
                          isDark ? 'hover:bg-zinc-700' : 'hover:bg-zinc-200'
                        }`}
                        title={t('resources.storage.download')}
                      >
                        {downloadingKey === file.key ? (
                          <Loader2 size={14} className="animate-spin text-amber-400" />
                        ) : (
                          <Download
                            size={14}
                            className={isDark ? 'text-zinc-400' : 'text-zinc-500'}
                          />
                        )}
                      </button>
                      <button
                        onClick={() => setDeleteTarget(file.key)}
                        className={`p-1.5 rounded-lg transition-colors ${
                          isDark ? 'hover:bg-red-500/10' : 'hover:bg-red-50'
                        }`}
                        title={t('resources.storage.delete')}
                      >
                        <Trash2 size={14} className="text-red-400" />
                      </button>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </GlassCard>

      {/* Delete confirm */}
      <ConfirmDialog
        open={!!deleteTarget}
        title={t('resources.storage.deleteDialog.title')}
        message={t('resources.storage.deleteDialog.message', { name: deleteTarget?.split('/').pop() })}
        confirmLabel={deleting ? t('resources.storage.deleteDialog.deleting') : t('resources.storage.deleteDialog.confirm')}
        variant="danger"
        loading={deleting}
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />
    </div>
  );
}
