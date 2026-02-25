import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { Search, RefreshCw, AlertTriangle, CheckCircle2, XCircle } from 'lucide-react';
import { API, normalizePaginatedData, showError } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import Card from './ui/Card';
import Badge from './ui/Badge';
import Button from './ui/Button';
import Input from './ui/Input';
import Pagination from './ui/Pagination';

const formatTime = (ts) => {
  if (!ts) {
    return '无';
  }
  return new Date(ts * 1000).toLocaleString();
};

const formatCost = (value) => {
  const amount = Number(value || 0);
  if (!Number.isFinite(amount) || amount <= 0) {
    return '$0.000000';
  }
  return `$${amount.toFixed(6)}`;
};

const formatUseTimeAndFirstToken = (log) => {
  const useTime = Number(log?.response_time_ms || 0);
  const firstToken = Number(log?.first_token_ms || 0);
  const isStream = Boolean(log?.is_stream);
  const firstTokenText = isStream ? `${firstToken} ms` : '-';
  const streamText = isStream ? '流式' : '非流式';
  return `${useTime} ms / ${firstTokenText} (${streamText})`;
};

const renderCacheValue = (log) => {
  const cacheTokens = Number(log?.cache_tokens || 0);
  const creationTokens = Number(log?.cache_creation_tokens || 0);
  const creation5mTokens = Number(log?.cache_creation_5m_tokens || 0);
  const creation1hTokens = Number(log?.cache_creation_1h_tokens || 0);

  const creationTooltip = `5分钟: ${creation5mTokens}, 1小时: ${creation1hTokens}`;

  return (
    <div className='log-cache-tags'>
      <span className='log-cache-tag log-cache-hit'>{cacheTokens}</span>
      {creationTokens > 0 && (
        <span className='log-cache-tag log-cache-create' title={creationTooltip}>
          +{creationTokens}
        </span>
      )}
    </div>
  );
};

const buildSummaryFromLogs = (logs, isErrorLog) => {
  const total = logs.length;
  const successCount = logs.filter((log) => !isErrorLog(log)).length;
  const errorCount = total - successCount;
  const inputTokens = logs.reduce((sum, log) => sum + Number(log.prompt_tokens || 0), 0);
  const outputTokens = logs.reduce((sum, log) => sum + Number(log.completion_tokens || 0), 0);
  const cacheTokens = logs.reduce((sum, log) => sum + Number(log.cache_tokens || 0), 0);
  const totalCost = logs.reduce((sum, log) => sum + Number(log.cost_usd || 0), 0);
  const avgLatency = total
    ? Math.round(logs.reduce((sum, log) => sum + Number(log.response_time_ms || 0), 0) / total)
    : 0;
  return { total, successCount, errorCount, inputTokens, outputTokens, cacheTokens, totalCost, avgLatency };
};

const LogsTable = ({ selfOnly }) => {
  const [logs, setLogs] = useState([]);
  const [total, setTotal] = useState(0);
  const [totalPages, setTotalPages] = useState(0);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [expandedRowId, setExpandedRowId] = useState(null);
  const [keyword, setKeyword] = useState('');
  const [providerFilter, setProviderFilter] = useState('all');
  const [providerOptions, setProviderOptions] = useState([]);
  const [statusFilter, setStatusFilter] = useState('all');
  const [viewTab, setViewTab] = useState('all');
  const [serverSummary, setServerSummary] = useState(null);
  const isErrorLog = useCallback(
    (log) =>
      Number(log?.status) !== 1 || (log?.error_message && String(log.error_message).trim() !== ''),
    []
  );

  const loadLogs = useCallback(async () => {
    setLoading(true);
    try {
      const endpoint = selfOnly ? '/api/log/self' : '/api/log/';
      const params = new URLSearchParams();
      params.set('p', String(page));
      params.set('page_size', String(ITEMS_PER_PAGE));
      const cleanKeyword = keyword.trim();
      if (cleanKeyword) {
        params.set('keyword', cleanKeyword);
      }
      if (providerFilter !== 'all') {
        params.set('provider', providerFilter);
      }
      if (statusFilter !== 'all') {
        params.set('status', statusFilter);
      }
      if (viewTab !== 'all') {
        params.set('view', viewTab);
      }
      const res = await API.get(`${endpoint}?${params.toString()}`);
      const { success, data, message } = res.data;
      if (success) {
        const normalized = normalizePaginatedData(data, { p: page, page_size: ITEMS_PER_PAGE });
        const pageItems = Array.isArray(normalized.items) ? normalized.items : [];
        setLogs(pageItems);
        setTotal(Number(normalized.total || 0));
        setTotalPages(Number(normalized.total_pages || 0));

        const providers = Array.isArray(normalized.providers)
          ? normalized.providers
          : Array.from(new Set(pageItems.map((log) => log.provider_name).filter(Boolean)));
        setProviderOptions(
          providers
            .filter(Boolean)
            .map((item) => String(item))
            .sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'))
        );

        const nextSummary = normalized.summary;
        if (nextSummary && typeof nextSummary === 'object') {
          setServerSummary({
            total: Number(nextSummary.total || 0),
            successCount: Number(nextSummary.success_count || 0),
            errorCount: Number(nextSummary.error_count || 0),
            inputTokens: Number(nextSummary.input_tokens || 0),
            outputTokens: Number(nextSummary.output_tokens || 0),
            cacheTokens: Number(nextSummary.cache_tokens || 0),
            totalCost: Number(nextSummary.total_cost || 0),
            avgLatency: Number(nextSummary.avg_latency || 0)
          });
        } else {
          setServerSummary(null);
        }
      } else {
        showError(message);
      }
    } catch (e) {
      showError('加载日志失败');
    } finally {
      setLoading(false);
    }
  }, [page, selfOnly, keyword, providerFilter, statusFilter, viewTab]);

  useEffect(() => {
    setPage(0);
  }, [keyword, providerFilter, statusFilter, viewTab]);

  useEffect(() => {
    loadLogs();
  }, [loadLogs]);

  const providers = useMemo(() => {
    const options = [...providerOptions];
    if (providerFilter !== 'all' && !options.includes(providerFilter)) {
      options.push(providerFilter);
    }
    return options.sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'));
  }, [providerOptions, providerFilter]);

  const pageSummary = useMemo(() => buildSummaryFromLogs(logs, isErrorLog), [logs, isErrorLog]);
  const summary = serverSummary || pageSummary;

  const toggleExpand = (id) => {
    setExpandedRowId(expandedRowId === id ? null : id);
  };

  const extractJsonFromText = (text) => {
    if (!text) return null;
    const start = text.indexOf('{');
    const end = text.lastIndexOf('}');
    if (start === -1 || end === -1 || end <= start) return null;
    const maybeJson = text.slice(start, end + 1);
    try {
      return JSON.parse(maybeJson);
    } catch (e) {
      return null;
    }
  };

  const parseErrorDetail = (log) => {
    const raw = String(log?.error_message || '');
    const detail = {
      request_id: log?.request_id || '',
      provider: log?.provider_name || '',
      model: log?.model_name || '',
      status: log?.status,
      created_at: formatTime(log?.created_at),
      raw_error_message: raw
    };

    const bodyTag = '\nrequest body:';
    const bodyIdx = raw.indexOf(bodyTag);
    if (bodyIdx >= 0) {
      const upstreamPart = raw.slice(0, bodyIdx).trim();
      const bodyPart = raw.slice(bodyIdx + bodyTag.length).trim();
      detail.upstream_error = extractJsonFromText(upstreamPart) || upstreamPart;
      detail.request_body = extractJsonFromText(bodyPart) || bodyPart;
      return detail;
    }

    detail.upstream_error = extractJsonFromText(raw) || raw;
    return detail;
  };

  const openErrorRawView = (log) => {
    const detail = parseErrorDetail(log);
    const storageKey = `raw_error_${Date.now()}_${Math.random().toString(36).slice(2, 10)}`;
    localStorage.setItem(storageKey, JSON.stringify(detail));

    const configuredServerAddress = String(localStorage.getItem('server_address') || '').trim();
    const fallbackBase = 'http://localhost:3000';
    const base = (configuredServerAddress || fallbackBase).replace(/\/+$/, '');
    const url = `${base}/log/raw?key=${encodeURIComponent(storageKey)}`;
    const win = window.open(url, '_blank');
    if (!win) {
      showError('浏览器拦截了新窗口，请允许弹窗后重试');
      return;
    }
  };

  const onPaginationChange = (e, { activePage: nextActivePage }) => {
    if (!Number.isFinite(Number(nextActivePage))) return;
    const normalizedPage = Math.max(1, Number(nextActivePage));
    const safeTotalPages = Math.max(Number(totalPages || 0), 1);
    if (normalizedPage > safeTotalPages) return;
    setPage(normalizedPage - 1);
  };

  return (
    <Card padding='0'>
      <div className='logs-header'>
        <div>
          <div className='logs-title'>{selfOnly ? '我的调用日志' : '全部调用日志'}</div>
          <div className='logs-subtitle'>直观卡片视图，支持供应商和错误日志快速筛选</div>
        </div>
        <Button
          variant='secondary'
          size='sm'
          icon={RefreshCw}
          onClick={loadLogs}
          disabled={loading}
        >
          刷新
        </Button>
      </div>

      <div className='logs-filter-bar'>
        <Input
          icon={Search}
          placeholder='搜索模型 / 供应商 / request id / error'
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
          style={{ marginBottom: 0, flex: 1, minWidth: '220px' }}
        />
        <select
          className='filter-select'
          value={providerFilter}
          onChange={(e) => setProviderFilter(e.target.value)}
        >
          <option value='all'>全部供应商</option>
          {providers.map((provider) => (
            <option key={provider} value={provider}>
              {provider}
            </option>
          ))}
        </select>
        <select
          className='filter-select'
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value='all'>全部状态</option>
          <option value='success'>仅成功</option>
          <option value='error'>仅失败</option>
        </select>
      </div>

      <div className='logs-tab-row'>
        <Button
          size='sm'
          variant={viewTab === 'all' ? 'primary' : 'secondary'}
          onClick={() => setViewTab('all')}
        >
          全部日志
        </Button>
        <Button
          size='sm'
          variant={viewTab === 'error' ? 'primary' : 'secondary'}
          onClick={() => setViewTab('error')}
          icon={AlertTriangle}
        >
          仅错误
        </Button>
      </div>

      <div className='logs-summary'>
        <Badge color='blue'>总计 {summary.total}</Badge>
        <Badge color='blue'>输入 {summary.inputTokens}</Badge>
        <Badge color='yellow'>输出 {summary.outputTokens}</Badge>
        <Badge color='gray'>缓存 {summary.cacheTokens}</Badge>
        <Badge color='green'>花费 {formatCost(summary.totalCost)}</Badge>
        <Badge color='green'>成功 {summary.successCount}</Badge>
        <Badge color='red'>失败 {summary.errorCount}</Badge>
        <Badge color='gray'>平均耗时 {summary.avgLatency}ms</Badge>
      </div>

      {loading ? (
        <div className='logs-empty'>加载中...</div>
      ) : logs.length === 0 ? (
        <div className='logs-empty'>当前筛选条件下没有日志记录</div>
      ) : (
        <div className='logs-card-list'>
          {logs.map((log) => (
            <div key={log.id} className='log-card'>
              <div className='log-card-top'>
                <div className='log-card-main'>
                  <code className='log-model-code'>{log.model_name || 'unknown-model'}</code>
                  <span className='log-provider'>@ {log.provider_name || '未知供应商'}</span>
                </div>
                <div className='log-card-state'>
                  {!isErrorLog(log) ? (
                    <Badge color='green'>
                      <CheckCircle2 size={12} style={{ marginRight: '0.25rem' }} />
                      成功
                    </Badge>
                  ) : (
                    <Badge color='red'>
                      <XCircle size={12} style={{ marginRight: '0.25rem' }} />
                      失败
                    </Badge>
                  )}
                  <span className='log-time'>{formatTime(log.created_at)}</span>
                </div>
              </div>

              <div className='log-meta-grid'>
                <div>
                  <span className='meta-label'>用时/首字（是否为流式）</span>
                  <span className='meta-value'>{formatUseTimeAndFirstToken(log)}</span>
                </div>
                <div>
                  <span className='meta-label'>输入</span>
                  <span className='meta-value'>{Number(log.prompt_tokens || 0)}</span>
                </div>
                <div>
                  <span className='meta-label'>输出</span>
                  <span className='meta-value'>{Number(log.completion_tokens || 0)}</span>
                </div>
                <div>
                  <span className='meta-label'>缓存</span>
                  <span className='meta-value'>{renderCacheValue(log)}</span>
                </div>
                <div>
                  <span className='meta-label'>花费</span>
                  <span className='meta-value'>{formatCost(log.cost_usd)}</span>
                </div>
                <div>
                  <span className='meta-label'>Request ID</span>
                  <span className='meta-value'>{log.request_id || '-'}</span>
                </div>
              </div>

              <div className='log-card-actions'>
                <Button size='sm' variant='ghost' onClick={() => toggleExpand(log.id)}>
                  {expandedRowId === log.id ? '收起详情' : '展开详情'}
                </Button>
              </div>

              {expandedRowId === log.id && (
                <>
                  {isErrorLog(log) && (
                    <div className='log-error-box'>
                      <div className='log-error-main'>
                        <AlertTriangle size={14} />
                        <span className='log-error-hint'>错误详情已隐藏，点击按钮在新页面查看</span>
                      </div>
                      <div className='log-error-action'>
                        <Button size='sm' variant='secondary' onClick={() => openErrorRawView(log)}>
                          查看 RAW 详情
                        </Button>
                      </div>
                    </div>
                  )}
                  <pre className='log-json-detail'>
                    {JSON.stringify(
                      {
                        ...log,
                        error_message: isErrorLog(log)
                          ? '[Hidden] 使用“查看 RAW 详情”按钮打开错误内容'
                          : log.error_message
                      },
                      null,
                      2
                    )}
                  </pre>
                </>
              )}
            </div>
          ))}
        </div>
      )}

      <div className='table-footer logs-pagination'>
        <span className='table-footer-meta'>共 {total} 条记录</span>
        <Pagination
          activePage={page + 1}
          totalPages={Math.max(totalPages, 1)}
          onPageChange={onPaginationChange}
        />
      </div>
    </Card>
  );
};

export default LogsTable;
