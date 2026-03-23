import React, { useCallback, useEffect, useState, useMemo } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
    ArrowLeft,
    RefreshCw,
    CheckSquare,
    Plus,
    Trash2,
    Edit,
    GitBranch
} from 'lucide-react';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { Table, Thead, Tbody, Tr, Th, Td } from '../../components/ui/Table';
import Button from '../../components/ui/Button';
import Card from '../../components/ui/Card';
import Badge from '../../components/ui/Badge';
import Modal from '../../components/ui/Modal';
import Input from '../../components/ui/Input';
import Tabs from '../../components/ui/Tabs';

const PROVIDER_TOKEN_KEY_STATUS_READY = 'ready';
const PROVIDER_TOKEN_KEY_STATUS_UNRESOLVED = 'unresolved';

const normalizeProviderTokenKeyStatus = (token) => {
    const status = String(token?.key_status || '').trim().toLowerCase();
    if (status === PROVIDER_TOKEN_KEY_STATUS_READY || status === PROVIDER_TOKEN_KEY_STATUS_UNRESOLVED) {
        return status;
    }
    const key = String(token?.sk_key || '').trim();
    if (key && !key.includes('**')) {
        return PROVIDER_TOKEN_KEY_STATUS_READY;
    }
    return PROVIDER_TOKEN_KEY_STATUS_UNRESOLVED;
};

const resolveProviderTokenCreateSuccessMessage = (message, outcome) => {
    const fallbackReadyMessage = 'Token 已在上游创建，密钥已同步，可在列表中复制';
    const fallbackUnresolvedMessage = 'Token 已在上游创建，但明文密钥暂未恢复，请稍后同步后重试';
    if (!outcome || !outcome.upstream_created) {
        return message || '令牌创建成功';
    }
    if (outcome.key_status === PROVIDER_TOKEN_KEY_STATUS_READY) {
        return message || fallbackReadyMessage;
    }
    return message || fallbackUnresolvedMessage;
};

const providerTokenUnresolvedReasonText = (reason) => {
    const normalizedReason = String(reason || '').trim().toLowerCase();
    if (normalizedReason === 'plaintext_not_recovered') {
        return '上游尚未返回可用明文密钥';
    }
    if (normalizedReason === 'legacy_contaminated') {
        return '历史密钥数据已污染，等待重新恢复';
    }
    if (normalizedReason === 'created_token_not_identified') {
        return '已创建上游令牌，但暂未定位到新令牌记录';
    }
    return '明文密钥暂未恢复';
};

const ProviderDetail = () => {
    const { id } = useParams();
    const navigate = useNavigate();
    const [provider, setProvider] = useState(null);
    const [tokens, setTokens] = useState([]);
    const [pricing, setPricing] = useState([]);
    const [pricingGroupRatio, setPricingGroupRatio] = useState({});
    const [endpointMap, setEndpointMap] = useState({});
    const [loading, setLoading] = useState(true);
    const [syncing, setSyncing] = useState(false);
    const [showTokenModal, setShowTokenModal] = useState(false);
    const [editToken, setEditToken] = useState(null);
    const [tokenGroupOptions, setTokenGroupOptions] = useState([]);
    const [defaultTokenGroup, setDefaultTokenGroup] = useState('');
    const [selectedPricing, setSelectedPricing] = useState(null);
    const [aliasMapping, setAliasMapping] = useState({});
    const [aliasMappingInput, setAliasMappingInput] = useState('{}');
    const [aliasLoading, setAliasLoading] = useState(false);
    const [aliasSaving, setAliasSaving] = useState(false);

    const loadProvider = useCallback(async () => {
        try {
            const res = await API.get(`/api/provider/${id}`);
            const { success, data, message } = res.data;
            if (success) setProvider(data);
            else showError(message);
        } catch (e) { showError('加载供应商失败'); }
    }, [id]);

    const loadTokens = useCallback(async () => {
        try {
            const res = await API.get(`/api/provider/${id}/tokens`);
            const { success, data, message } = res.data;
            if (success) setTokens(data || []);
            else showError(message);
        } catch (e) { showError('加载令牌失败'); }
    }, [id]);

    const loadPricing = useCallback(async () => {
        try {
            const res = await API.get(`/api/provider/${id}/pricing`);
            const { success, data, message, group_ratio, supported_endpoint, token_group_options, default_group } = res.data;
            if (success) {
                setPricing(data || []);
                setPricingGroupRatio(group_ratio || {});
                setEndpointMap(supported_endpoint || {});
                setTokenGroupOptions(Array.isArray(token_group_options) ? token_group_options : []);
                setDefaultTokenGroup(typeof default_group === 'string' ? default_group.trim() : '');
            }
            else showError(message);
        } catch (e) { showError('加载定价失败'); }
    }, [id]);

    const loadAliasMapping = useCallback(async () => {
        setAliasLoading(true);
        try {
            const res = await API.get(`/api/provider/${id}/model-alias-mapping`);
            const { success, data, message } = res.data;
            if (!success) {
                showError(message || '加载模型别名映射失败');
                return;
            }
            const mapping = (data && typeof data === 'object' && !Array.isArray(data)) ? data : {};
            setAliasMapping(mapping);
            setAliasMappingInput(JSON.stringify(mapping, null, 2));
        } catch (e) {
            showError('加载模型别名映射失败');
        } finally {
            setAliasLoading(false);
        }
    }, [id]);

    const loadAll = useCallback(async () => {
        setLoading(true);
        await Promise.all([loadProvider(), loadTokens(), loadPricing(), loadAliasMapping()]);
        setLoading(false);
    }, [loadAliasMapping, loadPricing, loadProvider, loadTokens]);

    useEffect(() => { loadAll(); }, [loadAll]);

    const syncProvider = async () => {
        setSyncing(true);
        try {
            const res = await API.post(`/api/provider/${id}/sync`);
            const { success, message } = res.data;
            if (success) {
                showSuccess('同步任务已启动，请稍后刷新查看结果');
                setTimeout(() => { loadAll(); setSyncing(false); }, 3000);
            } else { showError(message); setSyncing(false); }
        } catch (e) { showError('同步失败'); setSyncing(false); }
    };

    const checkinProvider = async () => {
        const res = await API.post(`/api/provider/${id}/checkin`);
        const { success, message } = res.data;
        if (success) { showSuccess(message || '签到成功'); loadProvider(); }
        else showError(message);
    };

    const getPreferredCreateGroup = useCallback((currentGroup = '') => {
        const normalizedCurrentGroup = String(currentGroup || '').trim();
        const availableGroups = new Set((tokenGroupOptions || []).map((item) => String(item.group_name || '').trim()).filter(Boolean));
        if (normalizedCurrentGroup && availableGroups.has(normalizedCurrentGroup)) {
            return normalizedCurrentGroup;
        }
        const normalizedDefaultGroup = String(defaultTokenGroup || '').trim();
        if (normalizedDefaultGroup && availableGroups.has(normalizedDefaultGroup)) {
            return normalizedDefaultGroup;
        }
        if (tokenGroupOptions.length > 0) {
            return String(tokenGroupOptions[0].group_name || '').trim();
        }
        return '';
    }, [defaultTokenGroup, tokenGroupOptions]);

    const openAddToken = () => {
        setEditToken({ name: '', group_name: getPreferredCreateGroup(''), status: 1, priority: 0, weight: 10, model_limits: '', unlimited_quota: true, remain_quota: 0 });
        setShowTokenModal(true);
    };

    const openEditToken = (token) => {
        setEditToken({
            ...token,
            group_name: String(token?.group_name || '').trim()
        });
        setShowTokenModal(true);
    };

    const saveToken = async () => {
        const selectedGroupName = String(editToken?.group_name || '').trim();
        const availableGroups = new Set((tokenGroupOptions || []).map((item) => String(item.group_name || '').trim()).filter(Boolean));

        if (!editToken?.id && availableGroups.size === 0) {
            showError('未获取到可用分组，请先同步供应商数据');
            return;
        }
        if (selectedGroupName === '') {
            showError('分组不能为空');
            return;
        }
        if (!editToken?.id && !availableGroups.has(selectedGroupName)) {
            showError('分组不属于该渠道可用分组，请先同步后重试');
            return;
        }
        const payload = { ...editToken, group_name: selectedGroupName };

        if (editToken.id) {
            const res = await API.put(`/api/provider/token/${editToken.id}`, payload);
            const { success, message } = res.data;
            if (success) { showSuccess('更新成功'); setShowTokenModal(false); loadTokens(); }
            else showError(message);
        } else {
            const res = await API.post(`/api/provider/${id}/tokens`, payload);
            const { success, message, data } = res.data;
            if (success) {
                showSuccess(resolveProviderTokenCreateSuccessMessage(message, data));
                setShowTokenModal(false);
                loadTokens();
            }
            else showError(message);
        }
    };

    const deleteToken = async (tokenId) => {
        if (!window.confirm('确定要删除此令牌吗？相关路由也会被删除。')) return;
        const res = await API.delete(`/api/provider/token/${tokenId}`);
        const { success, message } = res.data;
        if (success) { showSuccess('删除成功'); loadTokens(); }
        else showError(message);
    };

    const renderStatus = (status) => {
        if (status === 1) return <Badge color="green">启用</Badge>;
        return <Badge color="red">禁用</Badge>;
    };

    const copyTokenKey = async (tokenId, key) => {
        const normalizedKey = String(key || '').trim();
        if (!normalizedKey) {
            showError('当前令牌明文密钥未恢复，暂不可复制');
            return;
        }
        if (!navigator?.clipboard?.writeText) {
            showError('复制失败，请检查浏览器剪贴板权限');
            return;
        }
        try {
            await navigator.clipboard.writeText(normalizedKey);
            const btn = document.getElementById(`copy-btn-${tokenId}`);
            if (btn) {
                btn.textContent = '✓';
                setTimeout(() => {
                    btn.textContent = '复制';
                }, 1500);
            }
        } catch (e) {
            showError('复制失败，请检查浏览器剪贴板权限');
        }
    };

    const renderProviderHealth = (currentProvider) => {
        const status = String(currentProvider?.health_status || '').trim();
        if (currentProvider?.health_blocked || status === 'unreachable') {
            return <Badge color="red">不可用</Badge>;
        }
        if (status === 'healthy') {
            return <Badge color="green">可访问</Badge>;
        }
        return <Badge color="gray">未知</Badge>;
    };

    const formatTime = (timestamp) => {
        if (!timestamp) return '无';
        return timestamp2string(timestamp);
    };

    const parseEnableGroups = (enableGroups) => {
        try {
            const groups = JSON.parse(enableGroups || '[]');
            if (!Array.isArray(groups)) return [];
            return groups.filter((g) => typeof g === 'string' && g.trim() !== '');
        } catch (e) {
            return [];
        }
    };

    const parseSupportedEndpointTypes = (supportedEndpointTypes) => {
        try {
            const types = JSON.parse(supportedEndpointTypes || '[]');
            if (!Array.isArray(types)) return [];
            return types.filter((t) => typeof t === 'string' && t.trim() !== '');
        } catch (e) {
            return [];
        }
    };

    const resolveModelEndpoints = (pricingItem) => {
        const defaultEndpoint = [{ type: 'openai', path: '/v1/chat/completions', method: 'POST' }];
        if (!pricingItem) return defaultEndpoint;
        const endpointTypes = parseSupportedEndpointTypes(pricingItem.supported_endpoint_types);
        if (endpointTypes.length === 0) return defaultEndpoint;
        return endpointTypes.map((type) => {
            const info = endpointMap?.[type] || {};
            let path = info.path || '';
            if (path.includes('{model}')) {
                path = path.replaceAll('{model}', pricingItem.model_name || '');
            }
            return {
                type,
                path,
                method: info.method || 'POST'
            };
        });
    };

    const isPerRequestBilling = (pricingItem) => Number(pricingItem.model_price || 0) > 0 || Number(pricingItem.quota_type) === 1;

    const getGroupRatio = (groupName) => {
        const ratio = Number(pricingGroupRatio?.[groupName]);
        if (!Number.isFinite(ratio) || ratio <= 0) return 1;
        return ratio;
    };

    const getModelAvailableGroups = (pricingItem) => {
        const groups = parseEnableGroups(pricingItem?.enable_groups);
        if (groups.length > 0) return groups;
        return ['default'];
    };

    const calculatePromptPricePerMillion = (pricingItem, groupRatio = 1) => {
        if (isPerRequestBilling(pricingItem)) return null;
        return Number(pricingItem.model_ratio || 0) * 2 * groupRatio;
    };

    const calculateCompletionPricePerMillion = (pricingItem, groupRatio = 1) => {
        const promptPrice = calculatePromptPricePerMillion(pricingItem, groupRatio);
        if (promptPrice === null) return null;
        const completionRatio = Number(pricingItem.completion_ratio || 0);
        const ratio = completionRatio > 0 ? completionRatio : 1;
        return promptPrice * ratio;
    };

    const calculatePerCallPrice = (pricingItem, groupRatio = 1) => {
        if (!isPerRequestBilling(pricingItem)) return null;
        return Number(pricingItem.model_price || 0) * groupRatio;
    };

    const getBestGroupPricing = (pricingItem) => {
        const groups = getModelAvailableGroups(pricingItem);
        let usedGroup = groups[0];
        let usedGroupRatio = getGroupRatio(usedGroup);
        for (const g of groups) {
            const ratio = getGroupRatio(g);
            if (ratio < usedGroupRatio) {
                usedGroup = g;
                usedGroupRatio = ratio;
            }
        }
        return {
            groupName: usedGroup,
            groupRatio: usedGroupRatio,
            promptPrice: calculatePromptPricePerMillion(pricingItem, usedGroupRatio),
            completionPrice: calculateCompletionPricePerMillion(pricingItem, usedGroupRatio),
            perCallPrice: calculatePerCallPrice(pricingItem, usedGroupRatio)
        };
    };

    const formatPricePerMillion = (price) => {
        if (price === null || Number.isNaN(price)) return '-';
        return `$${price.toFixed(4)}`;
    };

    const formatRatio = (ratio) => {
        if (!Number.isFinite(ratio)) return '1';
        return ratio.toFixed(4).replace(/\.?0+$/, '');
    };

    const renderPerTokenPriceCell = (price) => {
        if (price === null || Number.isNaN(price)) {
            return <span style={{ color: 'var(--text-secondary)' }}>-</span>;
        }
        return (
            <div style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.2 }}>
                <span style={{ fontWeight: '600' }}>{formatPricePerMillion(price)}</span>
                <span style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>/ 1M tokens</span>
            </div>
        );
    };

    const renderPerCallPriceCell = (price) => {
        if (price === null || Number.isNaN(price)) {
            return <span style={{ color: 'var(--text-secondary)' }}>-</span>;
        }
        return (
            <div style={{ display: 'flex', flexDirection: 'column', lineHeight: 1.2 }}>
                <span style={{ fontWeight: '600' }}>{formatPricePerMillion(price)}</span>
                <span style={{ color: 'var(--text-secondary)', fontSize: '0.75rem' }}>/ 次</span>
            </div>
        );
    };

    // === Computed: Group → Model mapping from pricing data ===
    const groupModelMap = useMemo(() => {
        const map = {};
        for (const p of pricing) {
            const groups = parseEnableGroups(p.enable_groups);
            for (const g of groups) {
                if (!map[g]) map[g] = [];
                map[g].push(p);
            }
        }
        return map;
    }, [pricing]);

    // === Computed: Token Group names ===
    const tokenGroups = useMemo(() => {
        const groups = new Set();
        for (const t of tokens) {
            if (t.group_name) groups.add(t.group_name);
        }
        return [...groups];
    }, [tokens]);

    const tokenGroupSelectOptions = useMemo(() => {
        const options = (tokenGroupOptions || []).map((item) => ({
            group_name: String(item.group_name || '').trim(),
            ratio: Number(item.ratio),
            legacy: false
        })).filter((item) => item.group_name);
        const currentGroup = String(editToken?.group_name || '').trim();
        if (currentGroup && !options.some((item) => item.group_name === currentGroup)) {
            options.push({
                group_name: currentGroup,
                ratio: Number.NaN,
                legacy: true
            });
        }
        return options;
    }, [editToken?.group_name, tokenGroupOptions]);

    useEffect(() => {
        if (!showTokenModal) return;
        setEditToken((previous) => {
            if (!previous || previous.id) return previous;
            const preferredGroup = getPreferredCreateGroup(previous.group_name);
            if (preferredGroup === String(previous.group_name || '')) return previous;
            return { ...previous, group_name: preferredGroup };
        });
    }, [getPreferredCreateGroup, showTokenModal]);

    const aliasEntries = useMemo(() => {
        return Object.entries(aliasMapping || {})
            .map(([source, target]) => ({
                source: String(source || '').trim(),
                target: String(target || '').trim()
            }))
            .filter((item) => item.source && item.target)
            .sort((a, b) => a.source.localeCompare(b.source, 'zh-Hans-CN'));
    }, [aliasMapping]);

    const pricingModelNames = useMemo(() => {
        const names = new Set();
        for (const p of pricing) {
            const name = String(p.model_name || '').trim();
            if (name) names.add(name);
        }
        return [...names].sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'));
    }, [pricing]);

    const saveAliasMapping = async () => {
        let parsed;
        try {
            parsed = JSON.parse(aliasMappingInput || '{}');
        } catch (err) {
            showError('模型别名映射不是合法 JSON');
            return;
        }
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
            showError('模型别名映射必须是 JSON 对象');
            return;
        }
        const payload = {};
        Object.entries(parsed).forEach(([source, target]) => {
            const sourceKey = String(source || '').trim();
            const targetValue = String(target || '').trim();
            if (!sourceKey || !targetValue) return;
            payload[sourceKey] = targetValue;
        });

        setAliasSaving(true);
        try {
            const res = await API.put(`/api/provider/${id}/model-alias-mapping`, {
                model_alias_mapping: payload
            });
            const { success, data, message } = res.data;
            if (!success) {
                showError(message || '保存模型别名映射失败');
                return;
            }
            const nextMapping = (data && typeof data === 'object' && !Array.isArray(data)) ? data : payload;
            setAliasMapping(nextMapping);
            setAliasMappingInput(JSON.stringify(nextMapping, null, 2));
            showSuccess('模型别名映射已保存');
        } catch (e) {
            showError('保存模型别名映射失败');
        } finally {
            setAliasSaving(false);
        }
    };

    const formatAliasMapping = () => {
        try {
            const parsed = JSON.parse(aliasMappingInput || '{}');
            if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
                showError('模型别名映射必须是 JSON 对象');
                return;
            }
            setAliasMappingInput(JSON.stringify(parsed, null, 2));
        } catch (err) {
            showError('模型别名映射不是合法 JSON');
        }
    };

    if (loading) {
        return <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-secondary)' }}>加载中...</div>;
    }

    if (!provider) {
        return <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-secondary)' }}>供应商不存在</div>;
    }

    // ==============================================================
    //  Tab 1: Token Management
    // ==============================================================
    const tokenTab = (
        <>
            {tokens.length === 0 && (
                <Card style={{ marginBottom: '1rem', backgroundColor: 'var(--primary-50)', border: '1px solid var(--primary-200)' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                        <RefreshCw size={20} style={{ color: 'var(--primary-600)' }} />
                        <div>
                            <div style={{ fontWeight: '600', color: 'var(--primary-700)' }}>下一步：同步上游令牌</div>
                            <div style={{ fontSize: '0.875rem', color: 'var(--primary-600)', marginTop: '0.25rem' }}>
                                点击上方「同步」按钮，系统会自动从上游获取 API 令牌和模型信息，并生成路由。你也可以手动添加令牌。
                            </div>
                        </div>
                    </div>
                </Card>
            )}
            <Card padding="0">
                <div style={{ padding: '1rem', borderBottom: '1px solid var(--border-color)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div>
                        <div style={{ fontWeight: '600' }}>上游令牌列表</div>
                        <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>共 {tokens.length} 个令牌</div>
                    </div>
                    <div style={{ display: 'flex', gap: '0.5rem' }}>
                        <Button variant="secondary" onClick={loadTokens} icon={RefreshCw}>刷新</Button>
                        <Button variant="primary" onClick={openAddToken} icon={Plus}>创建上游令牌</Button>
                    </div>
                </div>
                {tokens.length === 0 ? (
                    <div style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-secondary)' }}>
                        <div style={{ marginBottom: '0.5rem', fontSize: '1rem' }}>暂无令牌</div>
                        <div style={{ fontSize: '0.875rem' }}>请点击「同步」从上游自动获取，或点击「创建上游令牌」在上游新增</div>
                    </div>
                ) : (
                    <Table>
                        <Thead>
                            <Tr>
                                <Th>编号</Th>
                                <Th>名称</Th>
                                <Th>密钥</Th>
                                <Th>分组</Th>
                                <Th>状态</Th>
                                <Th>配额</Th>
                                <Th>权重 / 优先级</Th>
                                <Th>操作</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {tokens.map((t) => {
                                const keyStatus = normalizeProviderTokenKeyStatus(t);
                                const keyReady = keyStatus === PROVIDER_TOKEN_KEY_STATUS_READY;
                                const unresolvedHint = providerTokenUnresolvedReasonText(t.key_unresolved_reason);
                                return (
                                    <Tr key={t.id}>
                                        <Td>{t.id}</Td>
                                        <Td>{t.name || '-'}</Td>
                                        <Td>
                                            {keyReady ? (
                                                <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                                                    <code style={{ fontSize: '0.8rem', backgroundColor: 'var(--gray-100)', padding: '0.15rem 0.4rem', borderRadius: '0.25rem', wordBreak: 'break-all', userSelect: 'all' }}>{t.sk_key}</code>
                                                    <button
                                                        onClick={() => copyTokenKey(t.id, t.sk_key)}
                                                        id={`copy-btn-${t.id}`}
                                                        style={{ fontSize: '0.7rem', padding: '0.15rem 0.4rem', borderRadius: '0.2rem', border: '1px solid var(--border-color)', backgroundColor: 'var(--bg-primary)', cursor: 'pointer', whiteSpace: 'nowrap', color: 'var(--text-secondary)' }}
                                                    >复制</button>
                                                </div>
                                            ) : (
                                                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                                                    <div style={{ display: 'flex', alignItems: 'center', gap: '0.35rem' }}>
                                                        <Badge color="yellow">待恢复</Badge>
                                                        <span style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>令牌已在上游创建，明文密钥暂不可复制</span>
                                                    </div>
                                                    <span style={{ fontSize: '0.75rem', color: 'var(--text-secondary)' }}>原因：{unresolvedHint}</span>
                                                </div>
                                            )}
                                        </Td>
                                        <Td>{t.group_name ? <Badge color="blue">{t.group_name}</Badge> : '-'}</Td>
                                        <Td>{renderStatus(t.status)}</Td>
                                        <Td>{t.unlimited_quota ? <Badge color="green">无限</Badge> : <span>{t.remain_quota}</span>}</Td>
                                        <Td>{t.weight} / {t.priority}</Td>
                                        <Td>
                                            <div style={{ display: 'flex', gap: '0.5rem' }}>
                                                <Button size="sm" variant="secondary" onClick={() => openEditToken(t)} title="编辑" icon={Edit} />
                                                <Button size="sm" variant="danger" onClick={() => deleteToken(t.id)} title="删除" icon={Trash2} />
                                            </div>
                                        </Td>
                                    </Tr>
                                );
                            })}
                        </Tbody>
                    </Table>
                )}
            </Card>
        </>
    );

    // ==============================================================
    //  Tab 2: Pricing & Models
    // ==============================================================
    const pricingTab = (
        <Card padding="0">
            <div style={{ padding: '1rem', borderBottom: '1px solid var(--border-color)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <div>
                    <div style={{ fontWeight: '600' }}>模型定价</div>
                    <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
                        价格按分组倍率计算，列表默认展示各模型可用分组中的最低价格，共 {pricing.length} 个模型
                    </div>
                </div>
                <Button variant="secondary" onClick={loadPricing} icon={RefreshCw}>刷新</Button>
            </div>
            {Object.keys(pricingGroupRatio || {}).length === 0 && (
                <div style={{ padding: '0.75rem 1rem', borderBottom: '1px solid var(--border-color)', color: 'var(--text-secondary)', fontSize: '0.875rem' }}>
                    未获取到分组倍率，当前按默认倍率 x1 计算
                </div>
            )}
            {pricing.length === 0 ? (
                <div style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-secondary)' }}>
                    <div style={{ marginBottom: '0.5rem', fontSize: '1rem' }}>暂无定价数据</div>
                    <div style={{ fontSize: '0.875rem' }}>请先同步供应商数据</div>
                </div>
            ) : (
                <Table>
                    <Thead>
                        <Tr>
                            <Th>模型名称</Th>
                            <Th>计费模式</Th>
                            <Th>提示</Th>
                            <Th>补全</Th>
                            <Th>单次</Th>
                            <Th>操作</Th>
                        </Tr>
                    </Thead>
                    <Tbody>
                        {pricing.map((p) => {
                            const isFixedPrice = isPerRequestBilling(p);
                            const bestGroupPricing = getBestGroupPricing(p);
                            return (
                                <Tr key={p.id}>
                                    <Td>
                                        <code style={{ fontSize: '0.8rem', backgroundColor: 'var(--gray-100)', padding: '0.15rem 0.4rem', borderRadius: '0.25rem' }}>
                                            {p.model_name}
                                        </code>
                                        <div style={{ marginTop: '0.35rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                                            分组：{bestGroupPricing.groupName}（x{formatRatio(bestGroupPricing.groupRatio)}）
                                        </div>
                                    </Td>
                                    <Td>
                                        {isFixedPrice ? (
                                            <Badge color="purple">按次</Badge>
                                        ) : (
                                            <Badge color="blue">按量</Badge>
                                        )}
                                    </Td>
                                    <Td>{renderPerTokenPriceCell(bestGroupPricing.promptPrice)}</Td>
                                    <Td>{renderPerTokenPriceCell(bestGroupPricing.completionPrice)}</Td>
                                    <Td>{renderPerCallPriceCell(bestGroupPricing.perCallPrice)}</Td>
                                    <Td><Button size="sm" variant="secondary" onClick={() => setSelectedPricing(p)}>详情</Button></Td>
                                </Tr>
                            );
                        })}
                    </Tbody>
                </Table>
            )}
        </Card>
    );

    // ==============================================================
    //  Tab 3: Group → Model Mapping
    // ==============================================================
    const groupTab = (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            {/* Legend */}
            <Card>
                <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                    <strong>关系说明：</strong>每个上游令牌属于一个「分组」，每个分组下可用一组模型（由上游定价的 <code>enable_groups</code> 决定）。
                    同步时会根据 <code>令牌.分组 → 定价.可用分组 → 模型</code> 的关系自动生成路由。
                </div>
                <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.5rem' }}>
                    <strong>令牌分组：</strong>{' '}
                    {tokenGroups.length === 0 ? '暂无' : tokenGroups.map((g, i) => (
                        <Badge key={i} color="blue" style={{ marginRight: '0.25rem' }}>{g}</Badge>
                    ))}
                </div>
            </Card>

            {Object.keys(groupModelMap).length === 0 ? (
                <Card>
                    <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-secondary)' }}>
                        暂无分组-模型映射数据，请先同步供应商
                    </div>
                </Card>
            ) : (
                Object.entries(groupModelMap).map(([group, models]) => {
                    const isActive = tokenGroups.includes(group);
                    return (
                        <Card key={group} padding="0" style={{ border: isActive ? '1px solid var(--primary-300)' : undefined }}>
                            <div style={{
                                padding: '0.75rem 1rem',
                                borderBottom: '1px solid var(--border-color)',
                                display: 'flex',
                                justifyContent: 'space-between',
                                alignItems: 'center',
                                backgroundColor: isActive ? 'var(--primary-50)' : undefined
                            }}>
                                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                                    <Badge color={isActive ? 'green' : 'gray'}>{group}</Badge>
                                    <span style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                                        {models.length} 个模型
                                    </span>
                                </div>
                                {isActive ? (
                                    <Badge color="green">有令牌属于此分组</Badge>
                                ) : (
                                    <Badge color="yellow">无令牌属于此分组</Badge>
                                )}
                            </div>
                            <div style={{ padding: '0.75rem 1rem', display: 'flex', flexWrap: 'wrap', gap: '0.4rem' }}>
                                {models.map((p, i) => (
                                    <code key={i} style={{
                                        fontSize: '0.8rem',
                                        backgroundColor: 'var(--gray-100)',
                                        padding: '0.2rem 0.5rem',
                                        borderRadius: '0.25rem',
                                        color: 'var(--text-primary)'
                                    }}>
                                        {p.model_name}
                                    </code>
                                ))}
                            </div>
                        </Card>
                    );
                })
            )}
        </div>
    );

    // ==============================================================
    //  Tab 4: Provider Model Alias Mapping
    // ==============================================================
    const aliasTab = (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
            <Card>
                <div style={{ fontWeight: '600' }}>模型别名映射（当前供应商）</div>
                <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.35rem', lineHeight: 1.5 }}>
                    用于把下游保留模型名映射到该供应商真实模型名。保存后，路由页会按保留模型名聚合展示，并在组内管理各路由权重。
                </div>
                <div style={{ marginTop: '0.75rem', fontSize: '0.82rem', color: 'var(--text-secondary)' }}>
                    JSON 格式（非常重要）：{`{"保留模型名":"上游真实模型名"}`}，例如 {`{"aaa":"bbbxxxcccddd"}`}
                </div>
                <textarea
                    rows={12}
                    value={aliasMappingInput}
                    onChange={(e) => setAliasMappingInput(e.target.value)}
                    placeholder='{"aaa":"provider-x/aaa-latest"}'
                    style={{
                        marginTop: '0.75rem',
                        width: '100%',
                        padding: '0.75rem',
                        borderRadius: 'var(--radius-md)',
                        border: '1px solid var(--border-color)',
                        backgroundColor: 'var(--bg-primary)',
                        color: 'var(--text-primary)',
                        fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
                        resize: 'vertical'
                    }}
                />
                <div style={{ marginTop: '0.75rem', display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
                    <Button variant="secondary" onClick={formatAliasMapping}>格式化 JSON</Button>
                    <Button variant="secondary" onClick={loadAliasMapping} icon={RefreshCw} disabled={aliasLoading}>刷新</Button>
                    <Button variant="primary" onClick={saveAliasMapping} loading={aliasSaving} disabled={aliasLoading}>保存映射</Button>
                </div>
                {pricingModelNames.length > 0 && (
                    <div style={{ marginTop: '0.85rem', fontSize: '0.8rem', color: 'var(--text-secondary)', lineHeight: 1.5 }}>
                        当前供应商可用目标模型（前 12 个）：{pricingModelNames.slice(0, 12).join('，')}{pricingModelNames.length > 12 ? ' ...' : ''}
                    </div>
                )}
            </Card>

            <Card padding="0">
                <div style={{ padding: '1rem', borderBottom: '1px solid var(--border-color)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ fontWeight: '600' }}>已生效映射</div>
                    <Badge color="blue">{aliasEntries.length} 条</Badge>
                </div>
                {aliasEntries.length === 0 ? (
                    <div style={{ padding: '1.25rem 1rem', color: 'var(--text-secondary)' }}>
                        暂无映射，当前仅使用自动归一化匹配。
                    </div>
                ) : (
                    <Table>
                        <Thead>
                            <Tr>
                                        <Th>保留模型名（对外）</Th>
                                        <Th>上游真实模型名（供应商）</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {aliasEntries.map((item) => (
                                <Tr key={`${item.source}=>${item.target}`}>
                                    <Td>
                                        <code style={{ fontSize: '0.8rem', backgroundColor: 'var(--gray-100)', padding: '0.15rem 0.4rem', borderRadius: '0.25rem' }}>
                                            {item.source}
                                        </code>
                                    </Td>
                                    <Td>
                                        <code style={{ fontSize: '0.8rem', backgroundColor: 'var(--gray-100)', padding: '0.15rem 0.4rem', borderRadius: '0.25rem' }}>
                                            {item.target}
                                        </code>
                                    </Td>
                                </Tr>
                            ))}
                        </Tbody>
                    </Table>
                )}
            </Card>
        </div>
    );

    return (
        <>
            {/* Header */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '1.5rem' }}>
                <Button variant="ghost" onClick={() => navigate('/provider')} icon={ArrowLeft}>返回</Button>
                <div>
                    <h2 style={{ fontSize: '1.5rem', fontWeight: 'bold', margin: 0 }}>{provider.name}</h2>
                    <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>{provider.base_url}</div>
                </div>
            </div>

            {/* Provider Info & Actions */}
            <Card style={{ marginBottom: '1.5rem' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: '1rem' }}>
                    <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap' }}>
                        <div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>状态</div>
                            {renderStatus(provider.status)}
                        </div>
                        <div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>余额</div>
                            <div style={{ fontWeight: '600' }}>{provider.balance || '无'}</div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '0.35rem' }}>
                                上次更新：{formatTime(provider.balance_updated)}
                            </div>
                        </div>
                        <div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>站点健康</div>
                            {renderProviderHealth(provider)}
                            {provider.health_failure_at ? (
                                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '0.35rem' }}>
                                    最近失败：{formatTime(provider.health_failure_at)}
                                </div>
                            ) : (
                                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '0.35rem' }}>
                                    最近成功：{formatTime(provider.health_success_at)}
                                </div>
                            )}
                        </div>
                        <div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>站点代理</div>
                            {provider.proxy_enabled ? <Badge color="orange">已启用</Badge> : <Badge color="gray">未启用</Badge>}
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '0.35rem', maxWidth: '220px' }}>
                                {provider.proxy_url_redacted || '直连'}
                            </div>
                        </div>
                        <div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>权重 / 优先级</div>
                            <div style={{ fontWeight: '600' }}>{provider.weight} / {provider.priority}</div>
                        </div>
                        <div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>签到</div>
                            {provider.checkin_enabled ? <Badge color="blue">已启用</Badge> : <Badge color="gray">未启用</Badge>}
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '0.35rem' }}>
                                上次成功：{formatTime(provider.last_checkin_at)}
                            </div>
                            {provider.last_checkin_status && (
                                <div style={{ fontSize: '0.75rem', color: provider.last_checkin_status === 'success' ? 'var(--success-color)' : 'var(--danger-color)', marginTop: '0.35rem' }}>
                                    最近结果：{provider.last_checkin_status}
                                </div>
                            )}
                        </div>
                        <div>
                            <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginBottom: '0.25rem' }}>令牌 / 模型</div>
                            <div style={{ fontWeight: '600' }}>{tokens.length} / {pricing.length}</div>
                        </div>
                    </div>
                    <div style={{ display: 'flex', gap: '0.5rem' }}>
                        <Button variant="primary" onClick={syncProvider} icon={RefreshCw} disabled={syncing}>
                            {syncing ? '同步中...' : '同步'}
                        </Button>
                        {provider.checkin_enabled && (
                            <Button variant="outline" onClick={checkinProvider} icon={CheckSquare}>签到</Button>
                        )}
                        <Button variant="outline" onClick={() => navigate('/routes')} icon={GitBranch}>查看路由</Button>
                    </div>
                </div>
                {provider.remark && (
                    <div style={{ marginTop: '1rem', padding: '0.75rem', backgroundColor: 'var(--gray-50)', borderRadius: 'var(--radius-md)', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                        {provider.remark}
                    </div>
                )}
                {provider.health_failure_reason && (
                    <div style={{ marginTop: '1rem', padding: '0.75rem', backgroundColor: 'rgba(239, 68, 68, 0.08)', borderRadius: 'var(--radius-md)', fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                        最近失败原因：{provider.health_failure_reason}
                    </div>
                )}
            </Card>

            {/* Tabs */}
            <Tabs items={[
                { label: `令牌管理 (${tokens.length})`, content: tokenTab },
                { label: `模型与定价 (${pricing.length})`, content: pricingTab },
                { label: `分组映射 (${Object.keys(groupModelMap).length})`, content: groupTab },
                { label: `模型别名映射 (${aliasEntries.length})`, content: aliasTab },
            ]} />

            <Modal
                title={selectedPricing ? `${selectedPricing.model_name} · 价格详情` : '价格详情'}
                isOpen={!!selectedPricing}
                onClose={() => setSelectedPricing(null)}
                actions={<Button variant="primary" onClick={() => setSelectedPricing(null)}>关闭</Button>}
            >
                {selectedPricing && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
                        <Card>
                            <div style={{ fontWeight: '600' }}>基本信息</div>
                            <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
                                模型的详细描述和基本特性
                            </div>
                            <div style={{ marginTop: '0.75rem', display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
                                <code style={{ fontSize: '0.8rem', backgroundColor: 'var(--gray-100)', padding: '0.15rem 0.4rem', borderRadius: '0.25rem' }}>
                                    {selectedPricing.model_name}
                                </code>
                                {isPerRequestBilling(selectedPricing) ? (
                                    <Badge color="purple">按次</Badge>
                                ) : (
                                    <Badge color="blue">按量</Badge>
                                )}
                            </div>
                            <div style={{ marginTop: '0.75rem', color: 'var(--text-secondary)', fontSize: '0.875rem' }}>暂无模型描述</div>
                        </Card>

                        <Card>
                            <div style={{ fontWeight: '600' }}>API端点</div>
                            <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
                                模型支持的接口端点信息
                            </div>
                            <div style={{ marginTop: '0.75rem' }}>
                                {(() => {
                                    const modelEndpoints = resolveModelEndpoints(selectedPricing);
                                    return (
                                        <div style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-md)', overflow: 'hidden' }}>
                                            {modelEndpoints.map((endpoint, idx) => (
                                                <div
                                                    key={`${endpoint.type}-${idx}`}
                                                    style={{
                                                        display: 'grid',
                                                        gridTemplateColumns: 'minmax(72px, 120px) 16px minmax(0, 1fr) auto',
                                                        alignItems: 'center',
                                                        columnGap: '0.5rem',
                                                        padding: '0.65rem 0.75rem',
                                                        borderBottom: idx === modelEndpoints.length - 1 ? 'none' : '1px dashed var(--border-color)'
                                                    }}
                                                >
                                                    <span style={{ color: 'var(--text-primary)', fontWeight: '500' }}>{endpoint.type || '-'}</span>
                                                    <span style={{ color: 'var(--text-secondary)', textAlign: 'center' }}>：</span>
                                                    <code style={{ fontSize: '0.8rem', color: 'var(--text-secondary)', overflowWrap: 'anywhere' }}>
                                                        {endpoint.path || '-'}
                                                    </code>
                                                    <span
                                                        style={{
                                                            fontSize: '0.75rem',
                                                            color: 'var(--text-secondary)',
                                                            padding: '0.1rem 0.45rem',
                                                            borderRadius: '999px',
                                                            border: '1px solid var(--border-color)',
                                                            textTransform: 'uppercase',
                                                            letterSpacing: '0.02em'
                                                        }}
                                                    >
                                                        {endpoint.method || 'POST'}
                                                    </span>
                                                </div>
                                            ))}
                                        </div>
                                    );
                                })()}
                            </div>
                        </Card>

                        <Card padding="0">
                            <div style={{ padding: '1rem', borderBottom: '1px solid var(--border-color)' }}>
                                <div style={{ fontWeight: '600' }}>分组价格</div>
                                <div style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginTop: '0.25rem' }}>
                                    不同用户分组的价格信息
                                </div>
                            </div>
                            <Table>
                                <Thead>
                                    <Tr>
                                        <Th>分组</Th>
                                        <Th>倍率</Th>
                                        <Th>计费类型</Th>
                                        <Th>提示</Th>
                                        <Th>补全</Th>
                                        <Th>单次</Th>
                                    </Tr>
                                </Thead>
                                <Tbody>
                                    {getModelAvailableGroups(selectedPricing).map((groupName) => {
                                        const ratio = getGroupRatio(groupName);
                                        return (
                                            <Tr key={groupName}>
                                                <Td>{groupName}</Td>
                                                <Td>x{formatRatio(ratio)}</Td>
                                                <Td>
                                                    {isPerRequestBilling(selectedPricing) ? (
                                                        <Badge color="purple">按次</Badge>
                                                    ) : (
                                                        <Badge color="blue">按量</Badge>
                                                    )}
                                                </Td>
                                                <Td>{renderPerTokenPriceCell(calculatePromptPricePerMillion(selectedPricing, ratio))}</Td>
                                                <Td>{renderPerTokenPriceCell(calculateCompletionPricePerMillion(selectedPricing, ratio))}</Td>
                                                <Td>{renderPerCallPriceCell(calculatePerCallPrice(selectedPricing, ratio))}</Td>
                                            </Tr>
                                        );
                                    })}
                                </Tbody>
                            </Table>
                        </Card>
                    </div>
                )}
            </Modal>

            {/* Token Modal */}
            <Modal
                title={editToken?.id ? '编辑令牌' : '在上游创建令牌'}
                isOpen={showTokenModal}
                onClose={() => setShowTokenModal(false)}
                closeOnOverlayClick={false}
                actions={
                    <Button variant="primary" onClick={saveToken} disabled={!editToken?.id && tokenGroupOptions.length === 0}>保存</Button>
                }
            >
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
                    <Input label="名称" placeholder="令牌名称" value={editToken?.name || ''} onChange={(e) => setEditToken({ ...editToken, name: e.target.value })} />
                    <div style={{ display: 'flex', flexDirection: 'column', marginBottom: '1rem' }}>
                        <label style={{ display: 'block', marginBottom: '0.5rem', fontSize: '0.875rem', fontWeight: '500', color: 'var(--text-secondary)' }}>分组名称</label>
                        <select
                            name="group_name"
                            value={editToken?.group_name || ''}
                            onChange={(e) => setEditToken({ ...editToken, group_name: e.target.value })}
                            style={{
                                width: '100%',
                                padding: '0.625rem 0.75rem',
                                fontSize: '0.875rem',
                                borderRadius: 'var(--radius-md)',
                                border: '1px solid var(--border-color)',
                                backgroundColor: 'var(--bg-primary)',
                                color: 'var(--text-primary)',
                                outline: 'none',
                            }}
                        >
                            <option value="">请选择分组</option>
                            {tokenGroupSelectOptions.map((option) => {
                                const ratioText = Number.isFinite(option.ratio) && option.ratio > 0 ? `x${formatRatio(option.ratio)}` : '未知倍率';
                                const legacySuffix = option.legacy ? '，历史分组' : '';
                                return (
                                    <option key={option.group_name} value={option.group_name}>
                                        {`${option.group_name} (${ratioText}${legacySuffix})`}
                                    </option>
                                );
                            })}
                        </select>
                        <span style={{ marginTop: '0.4rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
                            仅允许选择该渠道可用分组；默认优先上游默认分组，否则自动选择最低倍率分组。
                        </span>
                        {!editToken?.id && tokenGroupOptions.length === 0 && (
                            <span style={{ marginTop: '0.35rem', fontSize: '0.75rem', color: 'var(--danger-color)' }}>
                                未获取到可用分组，请先同步供应商数据
                            </span>
                        )}
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                        <Input label="权重" type="number" value={editToken?.weight || 10} onChange={(e) => setEditToken({ ...editToken, weight: parseInt(e.target.value) || 0 })} />
                        <Input label="优先级" type="number" value={editToken?.priority || 0} onChange={(e) => setEditToken({ ...editToken, priority: parseInt(e.target.value) || 0 })} />
                    </div>
                    <div style={{ display: 'flex', gap: '1rem' }}>
                        <div style={{ display: 'flex', alignItems: 'center' }}>
                            <input type="checkbox" id="token_status" checked={(editToken?.status || 0) === 1} onChange={(e) => setEditToken({ ...editToken, status: e.target.checked ? 1 : 0 })} style={{ marginRight: '0.5rem' }} />
                            <label htmlFor="token_status">启用</label>
                        </div>
                        <div style={{ display: 'flex', alignItems: 'center' }}>
                            <input type="checkbox" id="unlimited_quota" checked={editToken?.unlimited_quota || false} onChange={(e) => setEditToken({ ...editToken, unlimited_quota: e.target.checked })} style={{ marginRight: '0.5rem' }} />
                            <label htmlFor="unlimited_quota">无限配额</label>
                        </div>
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column' }}>
                        <label style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '0.5rem' }}>模型限制（逗号分隔，留空不限制）</label>
                        <textarea
                            rows={3}
                            value={editToken?.model_limits || ''}
                            onChange={(e) => setEditToken({ ...editToken, model_limits: e.target.value })}
                            style={{ padding: '0.5rem', borderRadius: 'var(--radius-md)', border: '1px solid var(--border-color)', width: '100%' }}
                        />
                    </div>
                </div>
            </Modal>
        </>
    );
};

export default ProviderDetail;
