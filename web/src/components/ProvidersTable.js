import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Plus,
  RefreshCw,
  Trash2,
  Edit,
  CheckSquare,
  Eye,
  Download,
  Upload
} from 'lucide-react';
import { API, showError, showSuccess, timestamp2string } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import { Table, Thead, Tbody, Tr, Th, Td } from './ui/Table';
import Button from './ui/Button';
import Card from './ui/Card';
import Badge from './ui/Badge';
import Modal from './ui/Modal';
import Input from './ui/Input';
import Pagination from './ui/Pagination';

const ACCESS_TOKEN_GUIDE_URL = 'https://github.com/xxbbzy/NewAPI-Gateway/blob/main/docs/provider-form-guide.md#access-token';
const UPSTREAM_USER_ID_GUIDE_URL = 'https://github.com/xxbbzy/NewAPI-Gateway/blob/main/docs/provider-form-guide.md#upstream-user-id';
const QUOTA_PER_USD = 500000;

const ProvidersTable = () => {
  const navigate = useNavigate();
  const [providers, setProviders] = useState([]);
  const [latestCheckinRun, setLatestCheckinRun] = useState(null);
  const [checkinMessages, setCheckinMessages] = useState([]);
  const [uncheckinProviders, setUncheckinProviders] = useState([]);
  const [checkinOverviewLoading, setCheckinOverviewLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [showModal, setShowModal] = useState(false);
  const [editProvider, setEditProvider] = useState(null);
  const [activePage, setActivePage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const inFlightPagesRef = useRef(new Set());
  const loadedPagesRef = useRef(new Set());
  const fetchEpochRef = useRef(0);

  const loadProviders = useCallback(async (startIdx = 0) => {
    // Prevent concurrent duplicated fetch for the same page.
    if (inFlightPagesRef.current.has(startIdx)) {
      return 0;
    }
    // Skip already loaded page (page 0 is reserved for full refresh).
    if (startIdx !== 0 && loadedPagesRef.current.has(startIdx)) {
      return 0;
    }

    const requestEpoch = fetchEpochRef.current;
    inFlightPagesRef.current.add(startIdx);
    setLoading(true);
    try {
      const res = await API.get(`/api/provider/?p=${startIdx}`);
      const { success, data, message } = res.data;
      // Ignore stale response from previous reload.
      if (requestEpoch !== fetchEpochRef.current) {
        return 0;
      }
      if (success) {
        const nextProviders = data || [];
        setHasMore(nextProviders.length === ITEMS_PER_PAGE);
        if (startIdx === 0) {
          loadedPagesRef.current = new Set([0]);
          setProviders(nextProviders);
        } else {
          loadedPagesRef.current.add(startIdx);
          setProviders((prevProviders) => {
            const seen = new Set(prevProviders.map((item) => item.id));
            const merged = [...prevProviders];
            nextProviders.forEach((item) => {
              if (!seen.has(item.id)) {
                seen.add(item.id);
                merged.push(item);
              }
            });
            return merged;
          });
        }
        return nextProviders.length;
      } else {
        showError(message);
      }
    } catch (e) {
      showError('加载供应商失败');
    } finally {
      inFlightPagesRef.current.delete(startIdx);
      setLoading(false);
    }
    return 0;
  }, []);

  const loadCheckinOverview = useCallback(async () => {
    setCheckinOverviewLoading(true);
    try {
      const [summaryRes, messageRes, uncheckinRes] = await Promise.all([
        API.get('/api/provider/checkin/summary?limit=1'),
        API.get('/api/provider/checkin/messages?limit=20'),
        API.get('/api/provider/checkin/uncheckin'),
      ]);
      const summaryBody = summaryRes.data || {};
      const messageBody = messageRes.data || {};
      const uncheckinBody = uncheckinRes.data || {};

      if (!summaryBody.success) {
        showError(summaryBody.message || '加载签到汇总失败');
      } else {
        setLatestCheckinRun((summaryBody.data || [])[0] || null);
      }
      if (!messageBody.success) {
        showError(messageBody.message || '加载签到消息失败');
      } else {
        setCheckinMessages(messageBody.data || []);
      }
      if (!uncheckinBody.success) {
        showError(uncheckinBody.message || '加载未签到渠道失败');
      } else {
        setUncheckinProviders(uncheckinBody.data || []);
      }
    } catch (error) {
      showError('加载签到概览失败');
    } finally {
      setCheckinOverviewLoading(false);
    }
  }, []);

  const reloadProviders = useCallback(async () => {
    fetchEpochRef.current += 1;
    inFlightPagesRef.current.clear();
    loadedPagesRef.current.clear();
    setActivePage(1);
    await Promise.all([loadProviders(0), loadCheckinOverview()]);
  }, [loadCheckinOverview, loadProviders]);

  useEffect(() => {
    loadProviders(0);
    loadCheckinOverview();
  }, [loadCheckinOverview, loadProviders]);

  const deleteProvider = async (id) => {
    if (!window.confirm('确定要删除此供应商吗？')) return;
    const res = await API.delete(`/api/provider/${id}`);
    const { success, message } = res.data;
    if (success) {
      showSuccess('删除成功');
      reloadProviders();
    } else {
      showError(message);
    }
  };

  const syncProvider = async (id) => {
    const res = await API.post(`/api/provider/${id}/sync`);
    const { success, message } = res.data;
    if (success) {
      showSuccess('同步任务已启动');
    } else {
      showError(message);
    }
  };

  const checkinProvider = async (id) => {
    const res = await API.post(`/api/provider/${id}/checkin`);
    const { success, message } = res.data;
    if (success) {
      showSuccess(message || '签到成功');
      reloadProviders();
    } else {
      showError(message);
    }
  };

  const runFullCheckin = async () => {
    const res = await API.post('/api/provider/checkin/run');
    const { success, message } = res.data;
    if (success) {
      showSuccess(message || '已触发全量签到');
      reloadProviders();
    } else {
      showError(message);
    }
  };

  const enableProviderCheckin = async (provider) => {
    const providerId = provider.id;
    const previousCheckinEnabled = !!provider.checkin_enabled;

    setProviders((prevProviders) => prevProviders.map((item) => (
      item.id === providerId ? { ...item, checkin_enabled: true } : item
    )));

    try {
      const res = await API.put('/api/provider/', {
        id: providerId,
        checkin_enabled: true,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(message || '签到已开启');
        reloadProviders();
        return;
      }
      setProviders((prevProviders) => prevProviders.map((item) => (
        item.id === providerId ? { ...item, checkin_enabled: previousCheckinEnabled } : item
      )));
      showError(message || '开启签到失败');
    } catch (error) {
      setProviders((prevProviders) => prevProviders.map((item) => (
        item.id === providerId ? { ...item, checkin_enabled: previousCheckinEnabled } : item
      )));
      showError('开启签到失败');
    }
  };

  const openEdit = (provider) => {
    setEditProvider({
      ...provider,
      user_id: provider?.user_id ? String(provider.user_id) : '',
    });
    setShowModal(true);
  };

  const openAdd = () => {
    setEditProvider({
      name: '',
      base_url: '',
      access_token: '',
      user_id: '',
      priority: 0,
      weight: 10,
      checkin_enabled: false,
      remark: '',
    });
    setShowModal(true);
  };

  const normalizeProviderPayload = () => ({
    ...editProvider,
    user_id: parseInt(editProvider?.user_id, 10) || 0,
  });

  const saveProvider = async () => {
    const providerPayload = normalizeProviderPayload();
    if (editProvider.id) {
      const res = await API.put('/api/provider/', providerPayload);
      const { success, message } = res.data;
      if (success) {
        showSuccess('更新成功');
        setShowModal(false);
        reloadProviders();
      } else {
        showError(message);
      }
    } else {
      const res = await API.post('/api/provider/', providerPayload);
      const { success, message } = res.data;
      if (success) {
        showSuccess('创建成功');
        setShowModal(false);
        reloadProviders();
      } else {
        showError(message);
      }
    }
  };

  const renderStatus = (status) => {
    if (status === 1) return <Badge color="green">启用</Badge>;
    return <Badge color="red">禁用</Badge>;
  };

  const renderCheckinResultStatus = (status) => {
    if (status === 'success') return <Badge color="green">成功</Badge>;
    if (status === 'partial') return <Badge color="orange">部分失败</Badge>;
    if (status === 'failed') return <Badge color="red">失败</Badge>;
    if (status === 'running') return <Badge color="blue">执行中</Badge>;
    return <Badge color="gray">{status || '未知'}</Badge>;
  };

  const isAutoDisabledCheckinItem = (item) => {
    if (item?.auto_disabled === true) {
      return true;
    }
    // Backward compatibility for historical records before auto_disabled field existed.
    const message = String(item?.message || '');
    return message.includes('已自动关闭签到');
  };

  const exportProviders = async () => {
    try {
      const res = await API.get('/api/provider/export');
      const { success, data, message } = res.data;
      if (success) {
        const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = 'providers.json';
        a.click();
        URL.revokeObjectURL(url);
        showSuccess('导出成功');
      } else {
        showError(message);
      }
    } catch (e) {
      showError('导出失败');
    }
  };

  const importProviders = () => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = '.json';
    input.onchange = async (e) => {
      const file = e.target.files[0];
      if (!file) return;
      try {
        const text = await file.text();
        const data = JSON.parse(text);
        const res = await API.post('/api/provider/import', data);
        const { success, message } = res.data;
        if (success) {
          showSuccess(message);
          reloadProviders();
        } else {
          showError(message);
        }
      } catch (err) {
        showError('JSON 解析失败: ' + err.message);
      }
    };
    input.click();
  };

  const formatTime = (timestamp) => {
    if (!timestamp) return '无';
    return timestamp2string(timestamp);
  };

  const formatCheckinReward = (quotaAwarded) => {
    const quota = Number(quotaAwarded || 0);
    if (!Number.isFinite(quota)) {
      return '$0.00';
    }
    return `$${(quota / QUOTA_PER_USD).toFixed(2)}`;
  };

  const onPaginationChange = async (e, { activePage: nextActivePage }) => {
    if (nextActivePage < 1) return;
    const loadedPages = Math.max(1, Math.ceil(providers.length / ITEMS_PER_PAGE));
    if (nextActivePage > loadedPages) {
      if (!hasMore) return;
      const loadedCount = await loadProviders(nextActivePage - 1);
      if (loadedCount === 0) return;
    }
    setActivePage(nextActivePage);
  };

  const displayedProviders = providers.slice(
    (activePage - 1) * ITEMS_PER_PAGE,
    activePage * ITEMS_PER_PAGE
  );
  const totalPages = Math.max(1, Math.ceil(providers.length / ITEMS_PER_PAGE) + (hasMore ? 1 : 0));

  return (
    <>
      <Card style={{ marginBottom: '1rem' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.75rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
          <div>
            <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>签到任务概览</div>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
              展示最近签到任务统计、结果消息与未签到渠道。
            </div>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <Button variant="outline" onClick={loadCheckinOverview} icon={RefreshCw} disabled={checkinOverviewLoading}>
              刷新概览
            </Button>
            <Button variant="primary" onClick={runFullCheckin} icon={CheckSquare} disabled={checkinOverviewLoading}>
              立即全量签到
            </Button>
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '0.75rem', marginBottom: '1rem' }}>
          <div style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.75rem' }}>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.75rem', marginBottom: '0.35rem' }}>最近任务状态</div>
            {latestCheckinRun ? renderCheckinResultStatus(latestCheckinRun.status) : <Badge color="gray">暂无数据</Badge>}
          </div>
          <div style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.75rem' }}>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.75rem', marginBottom: '0.35rem' }}>成功 / 失败</div>
            <div style={{ fontWeight: 600 }}>
              {latestCheckinRun ? `${latestCheckinRun.success_count} / ${latestCheckinRun.failure_count}` : '-'}
            </div>
          </div>
          <div style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.75rem' }}>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.75rem', marginBottom: '0.35rem' }}>未签到渠道</div>
            <div style={{ fontWeight: 600 }}>{uncheckinProviders.length}</div>
          </div>
          <div style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.75rem' }}>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.75rem', marginBottom: '0.35rem' }}>最近执行时间</div>
            <div style={{ fontWeight: 600 }}>
              {latestCheckinRun?.ended_at ? formatTime(latestCheckinRun.ended_at) : '无'}
            </div>
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '0.75rem' }}>
          <div style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.75rem' }}>
            <div style={{ fontWeight: 600, marginBottom: '0.5rem' }}>签到结果消息（最近 20 条）</div>
            <div style={{ maxHeight: '220px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              {checkinMessages.length === 0 ? (
                <div style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>暂无签到消息</div>
              ) : (
                checkinMessages.map((item) => {
                  const autoDisabledByUpstream = item.status === 'failed' && isAutoDisabledCheckinItem(item);
                  return (
                    <div key={item.id} style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.5rem' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.5rem' }}>
                        <div style={{ fontWeight: 600, fontSize: '0.875rem' }}>{item.provider_name || `供应商#${item.provider_id}`}</div>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                          {item.status === 'success' ? <Badge color="green">成功</Badge> : <Badge color="red">失败</Badge>}
                          {autoDisabledByUpstream && <Badge color="orange">已自动关闭签到</Badge>}
                        </div>
                      </div>
                      <div style={{ marginTop: '0.25rem', fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>{item.message || '-'}</div>
                      {autoDisabledByUpstream && (
                        <div style={{ marginTop: '0.25rem', fontSize: '0.75rem', color: 'var(--warning-color, #d97706)' }}>
                          签到功能上游未启用，已自动关闭该供应商签到
                        </div>
                      )}
                      <div style={{ marginTop: '0.25rem', fontSize: '0.75rem', color: 'var(--text-tertiary)' }}>
                        奖励额度：{formatCheckinReward(item.quota_awarded)} · {formatTime(item.checked_at)}
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          <div style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.75rem' }}>
            <div style={{ fontWeight: 600, marginBottom: '0.5rem' }}>未签到渠道（今日）</div>
            <div style={{ maxHeight: '220px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
              {uncheckinProviders.length === 0 ? (
                <div style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>今日所有已启用签到渠道均已签到</div>
              ) : (
                uncheckinProviders.map((provider) => (
                  <div key={provider.id} style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', padding: '0.5rem' }}>
                    <div style={{ fontWeight: 600, fontSize: '0.875rem' }}>{provider.name}</div>
                    <div style={{ marginTop: '0.25rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                      上次成功签到：{formatTime(provider.last_checkin_at)}
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </Card>

      <Card padding="0">
        <div style={{ padding: '1rem', borderBottom: '1px solid var(--border-color)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div style={{ fontWeight: '600' }}>供应商列表</div>
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <Button variant="outline" onClick={exportProviders} icon={Download}>导出</Button>
            <Button variant="outline" onClick={importProviders} icon={Upload}>导入</Button>
            <Button variant="primary" onClick={openAdd} icon={Plus}>添加供应商</Button>
          </div>
        </div>

        {loading ? (
          <div className="p-4 text-center text-gray-400" style={{ padding: '2rem', textAlign: 'center' }}>加载中...</div>
        ) : (
          <Table>
            <Thead>
              <Tr>
                <Th>编号</Th>
                <Th>名称</Th>
                <Th>地址</Th>
                <Th>加入时间</Th>
                <Th>状态</Th>
                <Th>余额</Th>
                <Th>权重</Th>
                <Th>优先级</Th>
                <Th>签到</Th>
                <Th>操作</Th>
              </Tr>
            </Thead>
            <Tbody>
              {displayedProviders.map((p, idx) => (
                <Tr key={p.id}>
                  <Td>{(activePage - 1) * ITEMS_PER_PAGE + idx + 1}</Td>
                  <Td>{p.name}</Td>
                  <Td><div style={{ maxWidth: '200px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={p.base_url}>{p.base_url}</div></Td>
                  <Td>{formatTime(p.created_at)}</Td>
                  <Td>{renderStatus(p.status)}</Td>
                  <Td>{p.balance ? p.balance : '无'}</Td>
                  <Td>{p.weight}</Td>
                  <Td>{p.priority}</Td>
                  <Td>
                    {p.checkin_enabled ? (
                      <Badge color="blue">已启用</Badge>
                    ) : (
                      <Badge color="gray">未启用</Badge>
                    )}
                  </Td>
                  <Td>
                    <div style={{ display: 'flex', gap: '0.5rem' }}>
                      <Button size="sm" variant="primary" onClick={() => navigate(`/provider/${p.id}`)} title="详情" icon={Eye} />
                      <Button size="sm" variant="secondary" onClick={() => openEdit(p)} title="编辑" icon={Edit} />
                      <Button size="sm" variant="outline" onClick={() => syncProvider(p.id)} title="同步" icon={RefreshCw} />
                      {!p.checkin_enabled && (
                        <Button size="sm" variant="ghost" color="blue" onClick={() => enableProviderCheckin(p)} title="一键开启签到" icon={CheckSquare} />
                      )}
                      {p.checkin_enabled && (
                        <Button size="sm" variant="ghost" color="green" onClick={() => checkinProvider(p.id)} title="签到" icon={CheckSquare} />
                      )}
                      <Button size="sm" variant="danger" onClick={() => deleteProvider(p.id)} title="删除" icon={Trash2} />
                    </div>
                  </Td>
                </Tr>
              ))}
              {displayedProviders.length === 0 && (
                <Tr>
                  <Td colSpan={10} style={{ textAlign: 'center', color: 'var(--text-muted)' }}>
                    暂无供应商数据
                  </Td>
                </Tr>
              )}
            </Tbody>
          </Table>
        )}

        {!loading && (
          <div style={{ padding: '1rem', borderTop: '1px solid var(--border-color)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '1rem', flexWrap: 'wrap' }}>
            <div style={{ color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
              已加载 {providers.length} 条记录
            </div>
            <Pagination
              activePage={activePage}
              onPageChange={onPaginationChange}
              totalPages={totalPages}
            />
          </div>
        )}
      </Card>

      <Modal
        title={editProvider?.id ? '编辑供应商' : '添加供应商'}
        isOpen={showModal}
        onClose={() => setShowModal(false)}
        closeOnOverlayClick={false}
        actions={
          <Button variant="primary" onClick={saveProvider}>保存</Button>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <Input
            label="名称"
            value={editProvider?.name || ''}
            onChange={(e) => setEditProvider({ ...editProvider, name: e.target.value })}
          />
          <Input
            label="基础地址"
            placeholder="https://api.example.com"
            value={editProvider?.base_url || ''}
            onChange={(e) => setEditProvider({ ...editProvider, base_url: e.target.value })}
          />
          <Input
            label="访问令牌"
            type="password"
            value={editProvider?.access_token || ''}
            onChange={(e) => setEditProvider({ ...editProvider, access_token: e.target.value })}
          />
          <div style={{ marginTop: '-0.5rem', marginBottom: '0.5rem', fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
            <a href={ACCESS_TOKEN_GUIDE_URL} target="_blank" rel="noreferrer" style={{ color: 'var(--primary-600)' }}>
              如何获取访问令牌
            </a>
          </div>
          <Input
            label="上游用户编号"
            type="number"
            value={editProvider?.user_id ?? ''}
            onFocus={() => {
              if (editProvider?.user_id === 0 || editProvider?.user_id === '0') {
                setEditProvider({ ...editProvider, user_id: '' });
              }
            }}
            onChange={(e) => setEditProvider({ ...editProvider, user_id: e.target.value })}
          />
          <div style={{ marginTop: '-0.5rem', marginBottom: '0.5rem', fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
            <a href={UPSTREAM_USER_ID_GUIDE_URL} target="_blank" rel="noreferrer" style={{ color: 'var(--primary-600)' }}>
              如何获取上游用户编号
            </a>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
            <Input
              label="权重"
              type="number"
              value={editProvider?.weight || 10}
              onChange={(e) => setEditProvider({ ...editProvider, weight: parseInt(e.target.value) || 0 })}
            />
            <Input
              label="优先级"
              type="number"
              value={editProvider?.priority || 0}
              onChange={(e) => setEditProvider({ ...editProvider, priority: parseInt(e.target.value) || 0 })}
            />
          </div>

          <div style={{ display: 'flex', alignItems: 'center', marginBottom: '1rem' }}>
            <input
              type="checkbox"
              id="checkin_enabled"
              checked={editProvider?.checkin_enabled || false}
              onChange={(e) => setEditProvider({ ...editProvider, checkin_enabled: e.target.checked })}
              style={{ marginRight: '0.5rem' }}
            />
            <label htmlFor="checkin_enabled">启用签到</label>
          </div>

          <div style={{ display: 'flex', flexDirection: 'column' }}>
            <label style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>备注</label>
            <textarea
              rows={3}
              value={editProvider?.remark || ''}
              onChange={(e) => setEditProvider({ ...editProvider, remark: e.target.value })}
              style={{
                padding: '0.5rem',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border-color)',
                width: '100%'
              }}
            />
          </div>
        </div>
      </Modal>
    </>
  );
};

export default ProvidersTable;
