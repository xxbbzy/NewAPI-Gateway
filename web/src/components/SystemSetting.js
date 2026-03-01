import React, { useEffect, useState } from 'react';
import { API, removeTrailingSlash, showError, showSuccess, timestamp2string } from '../helpers';
import Button from './ui/Button';
import Input from './ui/Input';
import Card from './ui/Card';

const WEBDAV_GUIDE_URL = 'https://github.com/xxbbzy/newapi-gateway/blob/main/docs/WEBDAV_SETTINGS_GUIDE.md';

const SystemSetting = () => {
  const [inputs, setInputs] = useState({
    PasswordLoginEnabled: '',
    PasswordRegisterEnabled: '',
    EmailVerificationEnabled: '',
    GitHubOAuthEnabled: '',
    GitHubClientId: '',
    GitHubClientSecret: '',
    Notice: '',
    SMTPServer: '',
    SMTPPort: '',
    SMTPAccount: '',
    SMTPToken: '',
    ServerAddress: '',
    Footer: '',
    TurnstileCheckEnabled: '',
    TurnstileSiteKey: '',
    TurnstileSecretKey: '',
    RegisterEnabled: '',
    CheckinScheduleEnabled: 'true',
    CheckinScheduleTime: '09:00',
    CheckinScheduleTimezone: 'Asia/Shanghai',
    RoutingUsageWindowHours: '24',
    RoutingBaseWeightFactor: '0.2',
    RoutingValueScoreFactor: '0.8',
    RoutingHealthAdjustmentEnabled: 'false',
    RoutingHealthWindowHours: '6',
    RoutingFailurePenaltyAlpha: '4.0',
    RoutingHealthRewardBeta: '0.08',
    RoutingHealthMinMultiplier: '0.05',
    RoutingHealthMaxMultiplier: '1.12',
    RoutingHealthMinSamples: '5',
    BackupEnabled: 'false',
    BackupTriggerMode: 'hybrid',
    BackupScheduleCron: '0 */6 * * *',
    BackupMinIntervalSeconds: '600',
    BackupDebounceSeconds: '30',
    BackupWebDAVURL: '',
    BackupWebDAVUsername: '',
    BackupWebDAVPassword: '',
    BackupWebDAVBasePath: '/newapi-gateway-backups',
    BackupEncryptEnabled: 'true',
    BackupEncryptPassphrase: '',
    BackupRetentionDays: '14',
    BackupRetentionMaxFiles: '100',
    BackupSpoolDir: 'upload/backup-spool',
    BackupCommandTimeoutSeconds: '600',
    BackupMaxRetries: '8',
    BackupRetryBaseSeconds: '30',
    BackupMySQLDumpCommand: 'mysqldump',
    BackupPostgresDumpCommand: 'pg_dump',
    BackupMySQLRestoreCommand: 'mysql',
    BackupPostgresRestoreCommand: 'psql',
  });
  const [originInputs, setOriginInputs] = useState({});
  const [loading, setLoading] = useState(false);
  const [backupStatus, setBackupStatus] = useState(null);
  const [backupRuns, setBackupRuns] = useState([]);
  const [backupRetries, setBackupRetries] = useState([]);
  const [backupPreflight, setBackupPreflight] = useState(null);
  const [restorePath, setRestorePath] = useState('');
  const [restoreConfirm, setRestoreConfirm] = useState('');
  const [restoreDryRunOk, setRestoreDryRunOk] = useState(false);
  const [restoreSummary, setRestoreSummary] = useState(null);
  const [restoreError, setRestoreError] = useState('');

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      const newInputs = {};
      data.forEach((item) => {
        newInputs[item.key] = item.value;
      });
      setInputs((prev) => ({ ...prev, ...newInputs }));
      setOriginInputs((prev) => ({ ...prev, ...newInputs }));
      const serverAddress = removeTrailingSlash(String(newInputs.ServerAddress || '').trim());
      if (serverAddress) {
        localStorage.setItem('server_address', serverAddress);
      } else {
        localStorage.removeItem('server_address');
      }
    } else {
      showError(message);
    }
  };

  const getBackupOverview = async () => {
    try {
      const [statusRes, runsRes, retriesRes] = await Promise.all([
        API.get('/api/backup/status'),
        API.get('/api/backup/runs?limit=10'),
        API.get('/api/backup/retries?limit=20'),
      ]);
      if (statusRes.data?.success) {
        setBackupStatus(statusRes.data.data || null);
        setBackupPreflight(statusRes.data.preflight || null);
      }
      if (runsRes.data?.success) {
        setBackupRuns(Array.isArray(runsRes.data.data) ? runsRes.data.data : []);
      }
      if (retriesRes.data?.success) {
        setBackupRetries(Array.isArray(retriesRes.data.data) ? retriesRes.data.data : []);
      }
    } catch (error) {
      showError(error);
    }
  };

  useEffect(() => {
    getOptions();
    getBackupOverview();
  }, []);

  const updateOption = async (key, value) => {
    setLoading(true);
    switch (key) {
      case 'PasswordLoginEnabled':
      case 'PasswordRegisterEnabled':
      case 'EmailVerificationEnabled':
      case 'GitHubOAuthEnabled':
      case 'TurnstileCheckEnabled':
      case 'RegisterEnabled':
      case 'CheckinScheduleEnabled':
      case 'RoutingHealthAdjustmentEnabled':
      case 'BackupEnabled':
      case 'BackupEncryptEnabled':
        value = inputs[key] === 'true' ? 'false' : 'true';
        break;
      default:
        break;
    }
    const res = await API.put('/api/option/', {
      key,
      value,
    });
    const { success, message } = res.data;
    if (success) {
      setInputs((current) => ({ ...current, [key]: value }));
      setOriginInputs((current) => ({ ...current, [key]: value }));
    } else {
      showError(message);
    }
    setLoading(false);
  };

  const handleInputChange = async (e) => {
    const { name, value } = e.target;
    if (
      name === 'Notice' ||
      name.startsWith('SMTP') ||
      name === 'ServerAddress' ||
      name === 'GitHubClientId' ||
      name === 'GitHubClientSecret' ||
      name === 'TurnstileSiteKey' ||
      name === 'TurnstileSecretKey' ||
      name === 'CheckinScheduleTime' ||
      name === 'CheckinScheduleTimezone' ||
      name === 'RoutingUsageWindowHours' ||
      name === 'RoutingBaseWeightFactor' ||
      name === 'RoutingValueScoreFactor' ||
      name === 'RoutingHealthWindowHours' ||
      name === 'RoutingFailurePenaltyAlpha' ||
      name === 'RoutingHealthRewardBeta' ||
      name === 'RoutingHealthMinMultiplier' ||
      name === 'RoutingHealthMaxMultiplier' ||
      name === 'RoutingHealthMinSamples' ||
      name === 'BackupTriggerMode' ||
      name === 'BackupScheduleCron' ||
      name === 'BackupMinIntervalSeconds' ||
      name === 'BackupDebounceSeconds' ||
      name === 'BackupWebDAVURL' ||
      name === 'BackupWebDAVUsername' ||
      name === 'BackupWebDAVPassword' ||
      name === 'BackupWebDAVBasePath' ||
      name === 'BackupEncryptPassphrase' ||
      name === 'BackupRetentionDays' ||
      name === 'BackupRetentionMaxFiles' ||
      name === 'BackupSpoolDir' ||
      name === 'BackupCommandTimeoutSeconds' ||
      name === 'BackupMaxRetries' ||
      name === 'BackupRetryBaseSeconds' ||
      name === 'BackupMySQLDumpCommand' ||
      name === 'BackupPostgresDumpCommand' ||
      name === 'BackupMySQLRestoreCommand' ||
      name === 'BackupPostgresRestoreCommand'
    ) {
      setInputs((current) => ({ ...current, [name]: value }));
    } else {
      await updateOption(name, value);
    }
  };

  const loginMethodGuardMessage = '至少保留一种登录方式（密码登录或 GitHub 登录）';
  const isPasswordLoginEnabled = inputs.PasswordLoginEnabled === 'true';
  const isGitHubOAuthEnabled = inputs.GitHubOAuthEnabled === 'true';
  const disablePasswordLoginToggle = isPasswordLoginEnabled && !isGitHubOAuthEnabled;
  const disableGitHubOAuthToggle = isGitHubOAuthEnabled && !isPasswordLoginEnabled;

  const handleCheckboxChange = async (name) => {
    if (
      (name === 'PasswordLoginEnabled' && disablePasswordLoginToggle) ||
      (name === 'GitHubOAuthEnabled' && disableGitHubOAuthToggle)
    ) {
      showError(loginMethodGuardMessage);
      return;
    }
    await updateOption(name, null);
  };

  const submitServerAddress = async () => {
    const serverAddress = removeTrailingSlash(inputs.ServerAddress);
    await updateOption('ServerAddress', serverAddress);
    if (serverAddress) {
      localStorage.setItem('server_address', serverAddress);
    } else {
      localStorage.removeItem('server_address');
    }
  };

  const submitSMTP = async () => {
    if (originInputs.SMTPServer !== inputs.SMTPServer) {
      await updateOption('SMTPServer', inputs.SMTPServer);
    }
    if (originInputs.SMTPAccount !== inputs.SMTPAccount) {
      await updateOption('SMTPAccount', inputs.SMTPAccount);
    }
    if (originInputs.SMTPPort !== inputs.SMTPPort && inputs.SMTPPort !== '') {
      await updateOption('SMTPPort', inputs.SMTPPort);
    }
    if (originInputs.SMTPToken !== inputs.SMTPToken && inputs.SMTPToken !== '') {
      await updateOption('SMTPToken', inputs.SMTPToken);
    }
  };

  const submitGitHubOAuth = async () => {
    if (originInputs.GitHubClientId !== inputs.GitHubClientId) {
      await updateOption('GitHubClientId', inputs.GitHubClientId);
    }
    if (originInputs.GitHubClientSecret !== inputs.GitHubClientSecret && inputs.GitHubClientSecret !== '') {
      await updateOption('GitHubClientSecret', inputs.GitHubClientSecret);
    }
  };

  const submitTurnstile = async () => {
    if (originInputs.TurnstileSiteKey !== inputs.TurnstileSiteKey) {
      await updateOption('TurnstileSiteKey', inputs.TurnstileSiteKey);
    }
    if (originInputs.TurnstileSecretKey !== inputs.TurnstileSecretKey && inputs.TurnstileSecretKey !== '') {
      await updateOption('TurnstileSecretKey', inputs.TurnstileSecretKey);
    }
  };

  const submitRoutingTuning = async () => {
    const rawWindow = Number.parseInt(String(inputs.RoutingUsageWindowHours || '').trim(), 10);
    const rawBaseFactor = Number.parseFloat(String(inputs.RoutingBaseWeightFactor || '').trim());
    const rawValueFactor = Number.parseFloat(String(inputs.RoutingValueScoreFactor || '').trim());
    const rawHealthWindow = Number.parseInt(String(inputs.RoutingHealthWindowHours || '').trim(), 10);
    const rawPenaltyAlpha = Number.parseFloat(String(inputs.RoutingFailurePenaltyAlpha || '').trim());
    const rawRewardBeta = Number.parseFloat(String(inputs.RoutingHealthRewardBeta || '').trim());
    const rawMinMultiplier = Number.parseFloat(String(inputs.RoutingHealthMinMultiplier || '').trim());
    const rawMaxMultiplier = Number.parseFloat(String(inputs.RoutingHealthMaxMultiplier || '').trim());
    const rawMinSamples = Number.parseInt(String(inputs.RoutingHealthMinSamples || '').trim(), 10);

    if (!Number.isInteger(rawWindow) || rawWindow < 1 || rawWindow > 720) {
      showError('统计窗口必须是 1 到 720 小时');
      return;
    }
    if (!Number.isFinite(rawBaseFactor) || rawBaseFactor < 0 || rawBaseFactor > 10) {
      showError('基础权重系数必须在 0 到 10 之间');
      return;
    }
    if (!Number.isFinite(rawValueFactor) || rawValueFactor < 0 || rawValueFactor > 10) {
      showError('性价比系数必须在 0 到 10 之间');
      return;
    }
    if (!Number.isInteger(rawHealthWindow) || rawHealthWindow < 1 || rawHealthWindow > 720) {
      showError('健康统计窗口必须是 1 到 720 小时');
      return;
    }
    if (!Number.isFinite(rawPenaltyAlpha) || rawPenaltyAlpha < 0 || rawPenaltyAlpha > 20) {
      showError('故障惩罚系数必须在 0 到 20 之间');
      return;
    }
    if (!Number.isFinite(rawRewardBeta) || rawRewardBeta < 0 || rawRewardBeta > 2) {
      showError('健康奖励系数必须在 0 到 2 之间');
      return;
    }
    if (!Number.isFinite(rawMinMultiplier) || rawMinMultiplier < 0 || rawMinMultiplier > 10) {
      showError('健康最小倍率必须在 0 到 10 之间');
      return;
    }
    if (!Number.isFinite(rawMaxMultiplier) || rawMaxMultiplier < 0 || rawMaxMultiplier > 10) {
      showError('健康最大倍率必须在 0 到 10 之间');
      return;
    }
    if (rawMaxMultiplier < rawMinMultiplier) {
      showError('健康最大倍率不能小于最小倍率');
      return;
    }
    if (!Number.isInteger(rawMinSamples) || rawMinSamples < 1 || rawMinSamples > 1000) {
      showError('健康最小样本数必须是 1 到 1000 的整数');
      return;
    }

    const nextValues = {
      RoutingUsageWindowHours: String(rawWindow),
      RoutingBaseWeightFactor: String(rawBaseFactor),
      RoutingValueScoreFactor: String(rawValueFactor),
      RoutingHealthAdjustmentEnabled: inputs.RoutingHealthAdjustmentEnabled === 'true' ? 'true' : 'false',
      RoutingHealthWindowHours: String(rawHealthWindow),
      RoutingFailurePenaltyAlpha: String(rawPenaltyAlpha),
      RoutingHealthRewardBeta: String(rawRewardBeta),
      RoutingHealthMinMultiplier: String(rawMinMultiplier),
      RoutingHealthMaxMultiplier: String(rawMaxMultiplier),
      RoutingHealthMinSamples: String(rawMinSamples),
    };

    for (const key of Object.keys(nextValues)) {
      if (originInputs[key] !== nextValues[key]) {
        await updateOption(key, nextValues[key]);
      }
    }
  };

  const submitCheckinSchedule = async () => {
    const scheduleTime = String(inputs.CheckinScheduleTime || '').trim();
    const scheduleTimezone = String(inputs.CheckinScheduleTimezone || '').trim();

    if (!/^([01]\d|2[0-3]):([0-5]\d)$/.test(scheduleTime)) {
      showError('签到时间格式必须是 HH:mm（24 小时制）');
      return;
    }
    if (scheduleTimezone === '') {
      showError('签到时区不能为空（例如 Asia/Shanghai）');
      return;
    }

    if (originInputs.CheckinScheduleTime !== scheduleTime) {
      await updateOption('CheckinScheduleTime', scheduleTime);
    }
    if (originInputs.CheckinScheduleTimezone !== scheduleTimezone) {
      await updateOption('CheckinScheduleTimezone', scheduleTimezone);
    }
  };

  const submitBackupSettings = async () => {
    const backupFieldLabels = {
      BackupTriggerMode: '触发模式',
      BackupScheduleCron: 'Cron 表达式',
      BackupMinIntervalSeconds: '最小间隔',
      BackupDebounceSeconds: '去抖延迟',
      BackupWebDAVURL: 'WebDAV URL',
      BackupWebDAVBasePath: 'WebDAV 基础目录',
      BackupRetentionDays: '保留天数',
      BackupRetentionMaxFiles: '最大保留文件数',
      BackupSpoolDir: '本地队列目录',
      BackupCommandTimeoutSeconds: '命令超时',
      BackupMaxRetries: '最大重试次数',
      BackupRetryBaseSeconds: '重试基准秒数',
      BackupMySQLDumpCommand: 'MySQL dump 命令',
      BackupPostgresDumpCommand: 'Postgres dump 命令',
      BackupMySQLRestoreCommand: 'MySQL restore 命令',
      BackupPostgresRestoreCommand: 'Postgres restore 命令',
    };
    const webdavURL = String(inputs.BackupWebDAVURL || '').trim();
    const hasWebdavURL = webdavURL !== '';
    if (inputs.BackupEnabled === 'true' && !hasWebdavURL) {
      showError('已启用备份，但未填写 WebDAV URL。请填写同一个地址，系统会自动用于备份与恢复。');
      return;
    }
    if (hasWebdavURL && !/^https?:\/\//i.test(webdavURL)) {
      showError('WebDAV URL 格式不正确，仅支持 http/https，例如 https://dav.example.com/backup');
      return;
    }
    const backupKeys = [
      'BackupTriggerMode',
      'BackupScheduleCron',
      'BackupMinIntervalSeconds',
      'BackupDebounceSeconds',
      'BackupWebDAVURL',
      'BackupWebDAVUsername',
      'BackupWebDAVPassword',
      'BackupWebDAVBasePath',
      'BackupEncryptPassphrase',
      'BackupRetentionDays',
      'BackupRetentionMaxFiles',
      'BackupSpoolDir',
      'BackupCommandTimeoutSeconds',
      'BackupMaxRetries',
      'BackupRetryBaseSeconds',
      'BackupMySQLDumpCommand',
      'BackupPostgresDumpCommand',
      'BackupMySQLRestoreCommand',
      'BackupPostgresRestoreCommand',
    ];
    const optionalKeys = new Set(['BackupWebDAVUsername', 'BackupWebDAVPassword', 'BackupEncryptPassphrase']);
    for (const key of backupKeys) {
      const nextValue = String(inputs[key] || '').trim();
      if (nextValue === '' && !optionalKeys.has(key)) {
        showError(`${backupFieldLabels[key] || key} 不能为空，请补全后再保存`);
        return;
      }
      if (originInputs[key] !== nextValue) {
        await updateOption(key, nextValue);
      }
    }
    await getBackupOverview();
    showSuccess('备份设置已保存');
  };

  const triggerManualBackup = async () => {
    try {
      const res = await API.post('/api/backup/trigger?trigger=manual');
      if (res.data?.success) {
        showSuccess('已触发手动备份');
        await getBackupOverview();
      } else {
        showError(res.data?.message || '触发手动备份失败');
      }
    } catch (error) {
      showError(error);
    }
  };

  const latestRestoreCandidate = backupRuns.find((run) => {
    if (run?.status !== 'success') {
      return false;
    }
    return String(run.remote_path || '').trim() !== '';
  }) || backupRuns.find((run) => {
    if (run?.status !== 'success') {
      return false;
    }
    return String(run.artifact_path || '').trim() !== '';
  }) || null;

  const resolveRestorePayload = () => {
    if (latestRestoreCandidate?.id) {
      return { run_id: latestRestoreCandidate.id };
    }
    const localPath = String(restorePath || '').trim();
    if (localPath) {
      return { local_path: localPath };
    }
    return null;
  };

  const getNoRestoreCandidateMessage = () => {
    const hasWebdavURL = String(inputs.BackupWebDAVURL || '').trim() !== '';
    if (!hasWebdavURL) {
      return '未找到可用恢复候选。请先在“备份基础配置”填写 WebDAV URL（备份与恢复共用同一地址），或在高级配置填写手动恢复路径。';
    }
    return '未找到可用恢复候选。请先点击“刷新状态”获取最新备份，或在高级配置填写手动恢复路径。';
  };

  const runRestoreDryRun = async () => {
    setRestoreError('');
    const payload = resolveRestorePayload();
    if (!payload) {
      const message = getNoRestoreCandidateMessage();
      setRestoreDryRunOk(false);
      setRestoreError(message);
      showError(message);
      return;
    }
    try {
      const res = await API.post('/api/backup/restore/validate', {
        ...payload,
        dry_run: true,
      });
      if (res.data?.success) {
        const ready = Boolean(res.data.data?.ready);
        setRestoreDryRunOk(ready);
        if (ready) {
          setRestoreSummary({ type: 'ready', message: res.data.data?.message || 'dry-run 校验通过，可执行恢复' });
          showSuccess('dry-run 校验通过，可执行恢复');
        } else {
          const message = res.data.data?.message || 'dry-run 校验未通过';
          setRestoreError(message);
          setRestoreSummary({ type: 'error', message });
          showError(message);
        }
      } else {
        setRestoreDryRunOk(false);
        const message = res.data?.message || 'dry-run 校验失败';
        setRestoreError(message);
        setRestoreSummary({ type: 'error', message });
        showError(message);
      }
    } catch (error) {
      const message = String(error);
      setRestoreDryRunOk(false);
      setRestoreError(message);
      setRestoreSummary({ type: 'error', message });
      showError(error);
    }
  };

  const executeRestore = async () => {
    setRestoreError('');
    if (!restoreDryRunOk) {
      const message = '请先通过 dry-run 校验';
      setRestoreError(message);
      showError(message);
      return;
    }
    const confirmValue = String(restoreConfirm || '').trim().toUpperCase();
    if (confirmValue !== 'RESTORE') {
      const message = '请输入 RESTORE 以确认恢复';
      setRestoreError(message);
      showError(message);
      return;
    }
    const payload = resolveRestorePayload();
    if (!payload) {
      const message = getNoRestoreCandidateMessage();
      setRestoreError(message);
      showError(message);
      return;
    }
    try {
      const res = await API.post('/api/backup/restore', {
        ...payload,
        dry_run: false,
        confirm: true,
      });
      if (res.data?.success) {
        showSuccess('恢复任务执行完成，请确认服务状态');
        setRestoreSummary({
          type: 'success',
          message: res.data?.data?.message || '恢复任务执行完成',
          health: res.data?.data?.health || null,
        });
        setRestoreDryRunOk(false);
        await getBackupOverview();
      } else {
        const message = res.data?.message || '恢复执行失败';
        setRestoreError(message);
        setRestoreSummary({ type: 'error', message });
        showError(message);
      }
    } catch (error) {
      const message = String(error);
      setRestoreError(message);
      setRestoreSummary({ type: 'error', message });
      showError(error);
    }
  };

  const Checkbox = ({ label, name, checked, onChange, disabled = false }) => (
    <div style={{ display: 'flex', alignItems: 'center', marginBottom: '0.75rem' }}>
      <input
        type="checkbox"
        id={name}
        checked={checked}
        disabled={disabled}
        onChange={() => onChange(name)}
        style={{ marginRight: '0.5rem', cursor: disabled ? 'not-allowed' : 'pointer' }}
      />
      <label
        htmlFor={name}
        style={{ cursor: disabled ? 'not-allowed' : 'pointer', userSelect: 'none', opacity: disabled ? 0.6 : 1 }}
      >
        {label}
      </label>
    </div>
  );

  const healthMinSamplesHint = (() => {
    const parsed = Number.parseInt(String(inputs.RoutingHealthMinSamples || '').trim(), 10);
    if (Number.isInteger(parsed) && parsed > 0) {
      return parsed;
    }
    return 5;
  })();

  const restorePayload = resolveRestorePayload();
  const canExecuteRestore = Boolean(restorePayload) && restoreDryRunOk;

  return (
    <div style={{ maxWidth: '900px', margin: '0 auto', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      <Card padding="1.5rem">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>基础访问</h3>
        <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
          保留高频基础配置，降低日常维护时的信息噪音。
        </p>
        <div style={{ marginBottom: '1rem' }}>
          <Input
            label='服务器地址'
            placeholder='例如：https://yourdomain.com'
            value={inputs.ServerAddress}
            name='ServerAddress'
            onChange={handleInputChange}
          />
          <Button onClick={submitServerAddress} variant="secondary" disabled={loading}>更新服务器地址</Button>
        </div>

        <div style={{ borderTop: '1px solid var(--border-color)', margin: '1.5rem 0' }}></div>

        <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '1rem' }}>登录注册开关</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: '0.5rem' }}>
          <Checkbox
            checked={isPasswordLoginEnabled}
            label='允许通过密码进行登录'
            name='PasswordLoginEnabled'
            onChange={handleCheckboxChange}
            disabled={disablePasswordLoginToggle}
          />
          <Checkbox checked={inputs.PasswordRegisterEnabled === 'true'} label='允许通过密码进行注册' name='PasswordRegisterEnabled' onChange={handleCheckboxChange} />
          <Checkbox checked={inputs.EmailVerificationEnabled === 'true'} label='通过密码注册时需要进行邮箱验证' name='EmailVerificationEnabled' onChange={handleCheckboxChange} />
          <Checkbox checked={inputs.RegisterEnabled === 'true'} label='允许新用户注册 (拒绝新用户)' name='RegisterEnabled' onChange={handleCheckboxChange} />
        </div>
        {disablePasswordLoginToggle && (
          <p style={{ marginTop: '0.25rem', fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
            {loginMethodGuardMessage}
          </p>
        )}
      </Card>

      <Card padding="1.5rem">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>备份基础配置（WebDAV）</h3>
        <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
          只保留最关键项：启用开关、WebDAV 地址、凭证和加密口令。
        </p>
        <div style={{ backgroundColor: '#fffbeb', border: '1px solid #fcd34d', color: '#92400e', padding: '0.75rem', borderRadius: 'var(--radius-md)', marginBottom: '1rem', fontSize: '0.875rem' }}>
          <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>简化规则</div>
          <div>你只需要配置同一个 WebDAV URL，系统会自动用于备份与恢复流程。</div>
          <a href={WEBDAV_GUIDE_URL} target='_blank' rel='noreferrer' style={{ color: '#92400e', textDecoration: 'underline' }}>
            查看 WebDAV 设置教程
          </a>
        </div>
        <div style={{ marginBottom: '1rem' }}>
          <Checkbox checked={inputs.BackupEnabled === 'true'} label='启用备份功能' name='BackupEnabled' onChange={handleCheckboxChange} />
          <Checkbox checked={inputs.BackupEncryptEnabled === 'true'} label='启用备份加密' name='BackupEncryptEnabled' onChange={handleCheckboxChange} />
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
          <Input label='WebDAV URL' name='BackupWebDAVURL' onChange={handleInputChange} value={inputs.BackupWebDAVURL} placeholder='https://example.com/dav' />
          <Input label='WebDAV 用户名' name='BackupWebDAVUsername' onChange={handleInputChange} value={inputs.BackupWebDAVUsername} />
          <Input label='WebDAV 密码' name='BackupWebDAVPassword' type='password' onChange={handleInputChange} value={inputs.BackupWebDAVPassword} />
          <Input label='加密口令（可空）' name='BackupEncryptPassphrase' type='password' onChange={handleInputChange} value={inputs.BackupEncryptPassphrase} />
        </div>
        <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap', marginBottom: '1rem' }}>
          <Button onClick={submitBackupSettings} variant="secondary" disabled={loading}>保存备份设置</Button>
          <Button onClick={triggerManualBackup} variant="secondary" disabled={loading || inputs.BackupEnabled !== 'true'}>立即备份</Button>
          <Button onClick={getBackupOverview} variant="outline" disabled={loading}>刷新状态</Button>
        </div>
        <div style={{ backgroundColor: 'var(--gray-50)', padding: '0.75rem', borderRadius: 'var(--radius-md)', fontSize: '0.875rem' }}>
          <div>预检状态：{backupPreflight?.ready ? '就绪' : '未就绪'}</div>
          <div>预检信息：{backupPreflight?.message || 'N/A'}</div>
          <div>最近运行状态：{backupStatus?.last_run?.status || 'N/A'}</div>
          <div>最近运行时间：{backupStatus?.last_run?.started_at ? timestamp2string(backupStatus.last_run.started_at) : 'N/A'}</div>
          <div>待重试队列：{Number(backupStatus?.pending_retry_count || 0)}</div>
          <div>最近重试记录：{backupRetries.length} 条</div>
        </div>
      </Card>

      <Card padding="1.5rem">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>恢复中心（高风险）</h3>
        <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
          默认使用最新可恢复备份，执行顺序为 Dry-Run 校验 - 输入 RESTORE - 执行恢复。
        </p>
        <div style={{ backgroundColor: 'var(--gray-50)', padding: '0.75rem', borderRadius: 'var(--radius-md)', marginBottom: '1rem', fontSize: '0.875rem' }}>
          <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>自动恢复候选</div>
          {latestRestoreCandidate ? (
            <>
              <div>RunID：{latestRestoreCandidate.id}</div>
              <div>来源字段：{latestRestoreCandidate.remote_path ? 'remote_path' : 'artifact_path'}</div>
              <div>候选路径：{latestRestoreCandidate.remote_path || latestRestoreCandidate.artifact_path}</div>
              <div>备份时间：{latestRestoreCandidate.started_at ? timestamp2string(latestRestoreCandidate.started_at) : 'N/A'}</div>
            </>
          ) : (
            <>
              <div>未找到自动候选。请先刷新状态，或在下方高级配置中填写手动恢复路径。</div>
              <a href={WEBDAV_GUIDE_URL} target='_blank' rel='noreferrer' style={{ color: 'var(--primary-600)', textDecoration: 'underline' }}>
                打开 WebDAV 设置教程
              </a>
            </>
          )}
        </div>
        {restoreError && (
          <div style={{ backgroundColor: '#fef2f2', border: '1px solid #fecaca', color: '#991b1b', padding: '0.75rem', borderRadius: 'var(--radius-md)', marginBottom: '1rem', fontSize: '0.875rem' }}>
            {restoreError}
          </div>
        )}
        {restoreSummary && (
          <div style={{
            backgroundColor: restoreSummary.type === 'success' ? '#ecfdf5' : restoreSummary.type === 'ready' ? '#eff6ff' : '#fef2f2',
            border: restoreSummary.type === 'success' ? '1px solid #a7f3d0' : restoreSummary.type === 'ready' ? '1px solid #bfdbfe' : '1px solid #fecaca',
            color: restoreSummary.type === 'success' ? '#065f46' : restoreSummary.type === 'ready' ? '#1d4ed8' : '#991b1b',
            padding: '0.75rem',
            borderRadius: 'var(--radius-md)',
            marginBottom: '1rem',
            fontSize: '0.875rem',
          }}>
            <div style={{ fontWeight: 600, marginBottom: '0.25rem' }}>
              {restoreSummary.type === 'success' ? '恢复执行成功' : restoreSummary.type === 'ready' ? 'Dry-Run 已通过' : '恢复流程出现错误'}
            </div>
            <div>{restoreSummary.message}</div>
            {restoreSummary.health && (
              <div style={{ marginTop: '0.25rem' }}>
                健康检查：users={Number(restoreSummary.health.users || 0)}，providers={Number(restoreSummary.health.providers || 0)}，options={Number(restoreSummary.health.options || 0)}
              </div>
            )}
          </div>
        )}
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
          <Input label='确认文本（输入 RESTORE）' name='restoreConfirm' onChange={(e) => setRestoreConfirm(e.target.value)} value={restoreConfirm} />
        </div>
        <div style={{ display: 'flex', gap: '0.75rem', flexWrap: 'wrap' }}>
          <Button onClick={runRestoreDryRun} variant="outline" disabled={loading}>
            {latestRestoreCandidate ? '先做自动候选 Dry-Run' : '先做手动路径 Dry-Run'}
          </Button>
          <Button onClick={executeRestore} variant="danger" disabled={loading || !canExecuteRestore}>执行恢复（需确认）</Button>
        </div>
      </Card>

      <details id='advanced-settings' style={{ border: '1px solid var(--border-color)', borderRadius: 'var(--radius-lg)', backgroundColor: 'var(--bg-primary)', padding: '1rem' }}>
        <summary style={{ cursor: 'pointer', fontWeight: 600 }}>高级配置（默认折叠）</summary>
        <div style={{ marginTop: '1rem' }}>
          <Card padding="1.5rem">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>登录扩展（高级）</h3>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: '0.5rem' }}>
              <Checkbox
                checked={isGitHubOAuthEnabled}
                label='允许通过 GitHub 账户登录 & 注册'
                name='GitHubOAuthEnabled'
                onChange={handleCheckboxChange}
                disabled={disableGitHubOAuthToggle}
              />
              <Checkbox checked={inputs.TurnstileCheckEnabled === 'true'} label='启用 Turnstile 用户校验' name='TurnstileCheckEnabled' onChange={handleCheckboxChange} />
            </div>
            {disableGitHubOAuthToggle && (
              <p style={{ marginTop: '0.25rem', fontSize: '0.8125rem', color: 'var(--text-secondary)' }}>
                {loginMethodGuardMessage}
              </p>
            )}
          </Card>

          <Card padding="1.5rem">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>备份高级参数与手动恢复回退</h3>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
              <Input label='触发模式(hybrid/event/schedule)' name='BackupTriggerMode' onChange={handleInputChange} value={inputs.BackupTriggerMode} />
              <Input label='Cron 表达式(5段)' name='BackupScheduleCron' onChange={handleInputChange} value={inputs.BackupScheduleCron} />
              <Input label='最小间隔(秒)' name='BackupMinIntervalSeconds' onChange={handleInputChange} value={inputs.BackupMinIntervalSeconds} />
              <Input label='去抖延迟(秒)' name='BackupDebounceSeconds' onChange={handleInputChange} value={inputs.BackupDebounceSeconds} />
              <Input label='WebDAV 基础目录' name='BackupWebDAVBasePath' onChange={handleInputChange} value={inputs.BackupWebDAVBasePath} />
              <Input label='保留天数' name='BackupRetentionDays' onChange={handleInputChange} value={inputs.BackupRetentionDays} />
              <Input label='最大保留文件数' name='BackupRetentionMaxFiles' onChange={handleInputChange} value={inputs.BackupRetentionMaxFiles} />
              <Input label='本地队列目录' name='BackupSpoolDir' onChange={handleInputChange} value={inputs.BackupSpoolDir} />
              <Input label='命令超时(秒)' name='BackupCommandTimeoutSeconds' onChange={handleInputChange} value={inputs.BackupCommandTimeoutSeconds} />
              <Input label='最大重试次数' name='BackupMaxRetries' onChange={handleInputChange} value={inputs.BackupMaxRetries} />
              <Input label='重试基准(秒)' name='BackupRetryBaseSeconds' onChange={handleInputChange} value={inputs.BackupRetryBaseSeconds} />
              <Input label='MySQL dump 命令' name='BackupMySQLDumpCommand' onChange={handleInputChange} value={inputs.BackupMySQLDumpCommand} />
              <Input label='Postgres dump 命令' name='BackupPostgresDumpCommand' onChange={handleInputChange} value={inputs.BackupPostgresDumpCommand} />
              <Input label='MySQL restore 命令' name='BackupMySQLRestoreCommand' onChange={handleInputChange} value={inputs.BackupMySQLRestoreCommand} />
              <Input label='Postgres restore 命令' name='BackupPostgresRestoreCommand' onChange={handleInputChange} value={inputs.BackupPostgresRestoreCommand} />
              <Input label='手动恢复路径（无自动候选时）' name='restorePath' onChange={(e) => setRestorePath(e.target.value)} value={restorePath} placeholder='/abs/path/to/backup.zip(.enc)' />
            </div>
            <Button onClick={submitBackupSettings} variant="secondary" disabled={loading}>保存高级备份参数</Button>
          </Card>

          <Card padding="1.5rem">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>签到任务设置</h3>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              配置每日自动签到时间和时区。系统会在配置时间点（及其后首次调度窗口）执行一次全量签到。
            </p>
            <div style={{ marginBottom: '1rem' }}>
              <Checkbox checked={inputs.CheckinScheduleEnabled === 'true'} label='启用每日自动签到任务' name='CheckinScheduleEnabled' onChange={handleCheckboxChange} />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
              <Input label='签到时间（HH:mm）' name='CheckinScheduleTime' onChange={handleInputChange} value={inputs.CheckinScheduleTime} placeholder='09:00' />
              <Input label='签到时区（IANA）' name='CheckinScheduleTimezone' onChange={handleInputChange} value={inputs.CheckinScheduleTimezone} placeholder='Asia/Shanghai' />
            </div>
            <Button onClick={submitCheckinSchedule} variant="secondary" disabled={loading}>保存签到任务设置</Button>
          </Card>

          <Card padding="1.5rem">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>路由策略调优（Beta）</h3>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              调整路由占比计算参数。占比贡献公式为：<code style={{ backgroundColor: 'var(--gray-200)', padding: '0.1rem 0.25rem' }}>max(weight+10,0) * (基础系数 + 性价比系数 * 归一化评分)</code>
            </p>
            <div style={{ marginBottom: '0.5rem', fontWeight: 600, color: 'var(--text-primary)' }}>性价比（金额）参数</div>
            <p style={{ fontSize: '0.8125rem', color: 'var(--text-secondary)', marginBottom: '0.75rem' }}>
              控制金额维度的占比分配，包括使用统计窗口、基础权重系数和性价比系数。
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
              <Input label='统计窗口（小时）' type='number' name='RoutingUsageWindowHours' onChange={handleInputChange} value={inputs.RoutingUsageWindowHours} min='1' max='720' step='1' placeholder='默认 24' />
              <Input label='基础权重系数' type='number' name='RoutingBaseWeightFactor' onChange={handleInputChange} value={inputs.RoutingBaseWeightFactor} min='0' max='10' step='0.1' placeholder='默认 0.2' />
              <Input label='性价比系数' type='number' name='RoutingValueScoreFactor' onChange={handleInputChange} value={inputs.RoutingValueScoreFactor} min='0' max='10' step='0.1' placeholder='默认 0.8' />
            </div>
            <div style={{ borderTop: '1px dashed var(--border-color)', margin: '1rem 0' }}></div>
            <div style={{ marginBottom: '0.5rem', fontWeight: 600, color: 'var(--text-primary)' }}>故障健康（Beta）参数</div>
            <div style={{ marginBottom: '0.75rem' }}>
              <Checkbox checked={inputs.RoutingHealthAdjustmentEnabled === 'true'} label='启用故障惩罚与健康奖励（Beta）' name='RoutingHealthAdjustmentEnabled' onChange={handleCheckboxChange} />
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
              <Input label='健康窗口（小时）' type='number' name='RoutingHealthWindowHours' onChange={handleInputChange} value={inputs.RoutingHealthWindowHours} min='1' max='720' step='1' placeholder='默认 6' />
              <Input label='故障惩罚系数 α' type='number' name='RoutingFailurePenaltyAlpha' onChange={handleInputChange} value={inputs.RoutingFailurePenaltyAlpha} min='0' max='20' step='0.1' placeholder='默认 4.0' />
              <Input label='健康奖励系数 β' type='number' name='RoutingHealthRewardBeta' onChange={handleInputChange} value={inputs.RoutingHealthRewardBeta} min='0' max='2' step='0.01' placeholder='默认 0.08' />
              <Input label='健康最小倍率' type='number' name='RoutingHealthMinMultiplier' onChange={handleInputChange} value={inputs.RoutingHealthMinMultiplier} min='0' max='10' step='0.01' placeholder='默认 0.05' />
              <Input label='健康最大倍率' type='number' name='RoutingHealthMaxMultiplier' onChange={handleInputChange} value={inputs.RoutingHealthMaxMultiplier} min='0' max='10' step='0.01' placeholder='默认 1.12' />
              <Input label='健康最小样本数' type='number' name='RoutingHealthMinSamples' onChange={handleInputChange} value={inputs.RoutingHealthMinSamples} min='1' max='1000' step='1' placeholder='默认 5' />
            </div>
            <p style={{ fontSize: '0.8125rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              健康调节仅当样本 &gt;= {healthMinSamplesHint} 时生效；不足阈值时按未调节（x1.000）处理。
            </p>
            <Button onClick={submitRoutingTuning} variant="secondary" disabled={loading}>保存路由策略参数</Button>
          </Card>

          <Card padding="1.5rem">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>配置 SMTP</h3>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>用以支持系统的邮件发送</p>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
              <Input label='SMTP 服务器地址' name='SMTPServer' onChange={handleInputChange} autoComplete='new-password' value={inputs.SMTPServer} placeholder='例如：smtp.qq.com' />
              <Input label='SMTP 端口' name='SMTPPort' onChange={handleInputChange} autoComplete='new-password' value={inputs.SMTPPort} placeholder='默认: 587' />
              <Input label='SMTP 账户' name='SMTPAccount' onChange={handleInputChange} autoComplete='new-password' value={inputs.SMTPAccount} placeholder='通常是邮箱地址' />
              <Input label='SMTP 访问凭证' name='SMTPToken' onChange={handleInputChange} type='password' autoComplete='new-password' value={inputs.SMTPToken} placeholder='敏感信息' />
            </div>
            <Button onClick={submitSMTP} variant="secondary" disabled={loading}>保存 SMTP 设置</Button>
          </Card>

          <Card padding="1.5rem">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>配置 GitHub OAuth 应用</h3>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              用以支持通过 GitHub 进行登录注册，
              <a href='https://github.com/settings/developers' target='_blank' rel="noreferrer" style={{ color: 'var(--primary-600)' }}> 点击此处 </a>
              管理你的 GitHub OAuth 应用
            </p>
            <div style={{ backgroundColor: 'var(--gray-50)', padding: '1rem', borderRadius: 'var(--radius-md)', marginBottom: '1rem', fontSize: '0.875rem' }}>
              首页地址填写 <code style={{ backgroundColor: 'var(--gray-200)', padding: '0.2rem' }}>{inputs.ServerAddress}</code>
              ，授权回调地址填写{' '}
              <code style={{ backgroundColor: 'var(--gray-200)', padding: '0.2rem' }}>{`${inputs.ServerAddress}/oauth/github`}</code>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
              <Input label='GitHub 客户端 ID' name='GitHubClientId' onChange={handleInputChange} autoComplete='new-password' value={inputs.GitHubClientId} placeholder='输入 ID' />
              <Input label='GitHub 客户端密钥' name='GitHubClientSecret' onChange={handleInputChange} type='password' autoComplete='new-password' value={inputs.GitHubClientSecret} placeholder='敏感信息' />
            </div>
            <Button onClick={submitGitHubOAuth} variant="secondary" disabled={loading}>保存 GitHub OAuth 设置</Button>
          </Card>

          <Card padding="1.5rem">
            <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>配置 Turnstile</h3>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              用以支持用户校验，
              <a href='https://dash.cloudflare.com/' target='_blank' rel="noreferrer" style={{ color: 'var(--primary-600)' }}> 点击此处 </a>
              管理你的 Turnstile 站点，推荐选择隐形组件类型
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
              <Input label='Turnstile 站点密钥' name='TurnstileSiteKey' onChange={handleInputChange} autoComplete='new-password' value={inputs.TurnstileSiteKey} placeholder='输入站点密钥' />
              <Input label='Turnstile 密钥' name='TurnstileSecretKey' onChange={handleInputChange} type='password' autoComplete='new-password' value={inputs.TurnstileSecretKey} placeholder='敏感信息' />
            </div>
            <Button onClick={submitTurnstile} variant="secondary" disabled={loading}>保存 Turnstile 设置</Button>
          </Card>
        </div>
      </details>
    </div>
  );
};

export default SystemSetting;
