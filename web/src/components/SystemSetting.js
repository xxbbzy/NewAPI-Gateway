import React, { useEffect, useState } from 'react';
import { API, removeTrailingSlash, showError } from '../helpers';
import Button from './ui/Button';
import Input from './ui/Input';
import Card from './ui/Card';

const SystemSetting = () => {
  let [inputs, setInputs] = useState({
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
  });
  const [originInputs, setOriginInputs] = useState({});
  let [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        newInputs[item.key] = item.value;
      });
      setInputs(newInputs);
      setOriginInputs(newInputs);
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

  useEffect(() => {
    getOptions();
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
      setInputs((inputs) => ({ ...inputs, [key]: value }));
      setOriginInputs((inputs) => ({ ...inputs, [key]: value }));
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
      name === 'RoutingHealthMinSamples'
    ) {
      setInputs((inputs) => ({ ...inputs, [name]: value }));
    } else {
      await updateOption(name, value);
    }
  };

  const handleCheckboxChange = async (name) => {
    await updateOption(name, null);
  };

  const submitServerAddress = async () => {
    let ServerAddress = removeTrailingSlash(inputs.ServerAddress);
    await updateOption('ServerAddress', ServerAddress);
    if (ServerAddress) {
      localStorage.setItem('server_address', ServerAddress);
    } else {
      localStorage.removeItem('server_address');
    }
  };

  const submitSMTP = async () => {
    if (originInputs['SMTPServer'] !== inputs.SMTPServer) {
      await updateOption('SMTPServer', inputs.SMTPServer);
    }
    if (originInputs['SMTPAccount'] !== inputs.SMTPAccount) {
      await updateOption('SMTPAccount', inputs.SMTPAccount);
    }
    if (
      originInputs['SMTPPort'] !== inputs.SMTPPort &&
      inputs.SMTPPort !== ''
    ) {
      await updateOption('SMTPPort', inputs.SMTPPort);
    }
    if (
      originInputs['SMTPToken'] !== inputs.SMTPToken &&
      inputs.SMTPToken !== ''
    ) {
      await updateOption('SMTPToken', inputs.SMTPToken);
    }
  };

  const submitGitHubOAuth = async () => {
    if (originInputs['GitHubClientId'] !== inputs.GitHubClientId) {
      await updateOption('GitHubClientId', inputs.GitHubClientId);
    }
    if (
      originInputs['GitHubClientSecret'] !== inputs.GitHubClientSecret &&
      inputs.GitHubClientSecret !== ''
    ) {
      await updateOption('GitHubClientSecret', inputs.GitHubClientSecret);
    }
  };

  const submitTurnstile = async () => {
    if (originInputs['TurnstileSiteKey'] !== inputs.TurnstileSiteKey) {
      await updateOption('TurnstileSiteKey', inputs.TurnstileSiteKey);
    }
    if (
      originInputs['TurnstileSecretKey'] !== inputs.TurnstileSecretKey &&
      inputs.TurnstileSecretKey !== ''
    ) {
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

    const nextWindow = String(rawWindow);
    const nextBaseFactor = String(rawBaseFactor);
    const nextValueFactor = String(rawValueFactor);
    const nextHealthEnabled = inputs.RoutingHealthAdjustmentEnabled === 'true' ? 'true' : 'false';
    const nextHealthWindow = String(rawHealthWindow);
    const nextPenaltyAlpha = String(rawPenaltyAlpha);
    const nextRewardBeta = String(rawRewardBeta);
    const nextMinMultiplier = String(rawMinMultiplier);
    const nextMaxMultiplier = String(rawMaxMultiplier);
    const nextMinSamples = String(rawMinSamples);

    if (originInputs['RoutingUsageWindowHours'] !== nextWindow) {
      await updateOption('RoutingUsageWindowHours', nextWindow);
    }
    if (originInputs['RoutingBaseWeightFactor'] !== nextBaseFactor) {
      await updateOption('RoutingBaseWeightFactor', nextBaseFactor);
    }
    if (originInputs['RoutingValueScoreFactor'] !== nextValueFactor) {
      await updateOption('RoutingValueScoreFactor', nextValueFactor);
    }
    if (originInputs['RoutingHealthAdjustmentEnabled'] !== nextHealthEnabled) {
      await updateOption('RoutingHealthAdjustmentEnabled', nextHealthEnabled);
    }
    if (originInputs['RoutingHealthWindowHours'] !== nextHealthWindow) {
      await updateOption('RoutingHealthWindowHours', nextHealthWindow);
    }
    if (originInputs['RoutingFailurePenaltyAlpha'] !== nextPenaltyAlpha) {
      await updateOption('RoutingFailurePenaltyAlpha', nextPenaltyAlpha);
    }
    if (originInputs['RoutingHealthRewardBeta'] !== nextRewardBeta) {
      await updateOption('RoutingHealthRewardBeta', nextRewardBeta);
    }
    if (originInputs['RoutingHealthMinMultiplier'] !== nextMinMultiplier) {
      await updateOption('RoutingHealthMinMultiplier', nextMinMultiplier);
    }
    if (originInputs['RoutingHealthMaxMultiplier'] !== nextMaxMultiplier) {
      await updateOption('RoutingHealthMaxMultiplier', nextMaxMultiplier);
    }
    if (originInputs['RoutingHealthMinSamples'] !== nextMinSamples) {
      await updateOption('RoutingHealthMinSamples', nextMinSamples);
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

    if (originInputs['CheckinScheduleTime'] !== scheduleTime) {
      await updateOption('CheckinScheduleTime', scheduleTime);
    }
    if (originInputs['CheckinScheduleTimezone'] !== scheduleTimezone) {
      await updateOption('CheckinScheduleTimezone', scheduleTimezone);
    }
  };

  const Checkbox = ({ label, name, checked, onChange }) => (
    <div style={{ display: 'flex', alignItems: 'center', marginBottom: '0.75rem' }}>
      <input
        type="checkbox"
        id={name}
        checked={checked}
        onChange={() => onChange(name)}
        style={{ marginRight: '0.5rem' }}
      />
      <label htmlFor={name} style={{ cursor: 'pointer', userSelect: 'none' }}>{label}</label>
    </div>
  );

  const healthMinSamplesHint = (() => {
    const parsed = Number.parseInt(String(inputs.RoutingHealthMinSamples || '').trim(), 10);
    if (Number.isInteger(parsed) && parsed > 0) {
      return parsed;
    }
    return 5;
  })();

  return (
    <div style={{ maxWidth: '800px', margin: '0 auto', display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
      <Card padding="1.5rem">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '1rem' }}>通用设置</h3>
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

        <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '1rem' }}>配置登录注册</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))', gap: '0.5rem' }}>
          <Checkbox
            checked={inputs.PasswordLoginEnabled === 'true'}
            label='允许通过密码进行登录'
            name='PasswordLoginEnabled'
            onChange={handleCheckboxChange}
          />
          <Checkbox
            checked={inputs.PasswordRegisterEnabled === 'true'}
            label='允许通过密码进行注册'
            name='PasswordRegisterEnabled'
            onChange={handleCheckboxChange}
          />
          <Checkbox
            checked={inputs.EmailVerificationEnabled === 'true'}
            label='通过密码注册时需要进行邮箱验证'
            name='EmailVerificationEnabled'
            onChange={handleCheckboxChange}
          />
          <Checkbox
            checked={inputs.GitHubOAuthEnabled === 'true'}
            label='允许通过 GitHub 账户登录 & 注册'
            name='GitHubOAuthEnabled'
            onChange={handleCheckboxChange}
          />
          <Checkbox
            checked={inputs.RegisterEnabled === 'true'}
            label='允许新用户注册 (拒绝新用户)'
            name='RegisterEnabled'
            onChange={handleCheckboxChange}
          />
          <Checkbox
            checked={inputs.TurnstileCheckEnabled === 'true'}
            label='启用 Turnstile 用户校验'
            name='TurnstileCheckEnabled'
            onChange={handleCheckboxChange}
          />
        </div>
      </Card>

      <Card padding="1.5rem">
        <h3 style={{ fontSize: '1.1rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>签到任务设置</h3>
        <p style={{ fontSize: '0.875rem', color: 'var(--text-secondary)', marginBottom: '1rem' }}>
          配置每日自动签到时间和时区。系统会在配置时间点（及其后首次调度窗口）执行一次全量签到。
        </p>
        <div style={{ marginBottom: '1rem' }}>
          <Checkbox
            checked={inputs.CheckinScheduleEnabled === 'true'}
            label='启用每日自动签到任务'
            name='CheckinScheduleEnabled'
            onChange={handleCheckboxChange}
          />
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
          <Input
            label='签到时间（HH:mm）'
            name='CheckinScheduleTime'
            onChange={handleInputChange}
            value={inputs.CheckinScheduleTime}
            placeholder='09:00'
          />
          <Input
            label='签到时区（IANA）'
            name='CheckinScheduleTimezone'
            onChange={handleInputChange}
            value={inputs.CheckinScheduleTimezone}
            placeholder='Asia/Shanghai'
          />
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
          <Input
            label='统计窗口（小时）'
            type='number'
            name='RoutingUsageWindowHours'
            onChange={handleInputChange}
            value={inputs.RoutingUsageWindowHours}
            min='1'
            max='720'
            step='1'
            placeholder='默认 24'
          />
          <Input
            label='基础权重系数'
            type='number'
            name='RoutingBaseWeightFactor'
            onChange={handleInputChange}
            value={inputs.RoutingBaseWeightFactor}
            min='0'
            max='10'
            step='0.1'
            placeholder='默认 0.2'
          />
          <Input
            label='性价比系数'
            type='number'
            name='RoutingValueScoreFactor'
            onChange={handleInputChange}
            value={inputs.RoutingValueScoreFactor}
            min='0'
            max='10'
            step='0.1'
            placeholder='默认 0.8'
          />
        </div>
        <div style={{ borderTop: '1px dashed var(--border-color)', margin: '1rem 0' }}></div>
        <div style={{ marginBottom: '0.5rem', fontWeight: 600, color: 'var(--text-primary)' }}>故障健康（Beta）参数</div>
        <div style={{ marginBottom: '0.75rem' }}>
          <Checkbox
            checked={inputs.RoutingHealthAdjustmentEnabled === 'true'}
            label='启用故障惩罚与健康奖励（Beta）'
            name='RoutingHealthAdjustmentEnabled'
            onChange={handleCheckboxChange}
          />
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))', gap: '1rem', marginBottom: '1rem' }}>
          <Input
            label='健康窗口（小时）'
            type='number'
            name='RoutingHealthWindowHours'
            onChange={handleInputChange}
            value={inputs.RoutingHealthWindowHours}
            min='1'
            max='720'
            step='1'
            placeholder='默认 6'
          />
          <Input
            label='故障惩罚系数 α'
            type='number'
            name='RoutingFailurePenaltyAlpha'
            onChange={handleInputChange}
            value={inputs.RoutingFailurePenaltyAlpha}
            min='0'
            max='20'
            step='0.1'
            placeholder='默认 4.0'
          />
          <Input
            label='健康奖励系数 β'
            type='number'
            name='RoutingHealthRewardBeta'
            onChange={handleInputChange}
            value={inputs.RoutingHealthRewardBeta}
            min='0'
            max='2'
            step='0.01'
            placeholder='默认 0.08'
          />
          <Input
            label='健康最小倍率'
            type='number'
            name='RoutingHealthMinMultiplier'
            onChange={handleInputChange}
            value={inputs.RoutingHealthMinMultiplier}
            min='0'
            max='10'
            step='0.01'
            placeholder='默认 0.05'
          />
          <Input
            label='健康最大倍率'
            type='number'
            name='RoutingHealthMaxMultiplier'
            onChange={handleInputChange}
            value={inputs.RoutingHealthMaxMultiplier}
            min='0'
            max='10'
            step='0.01'
            placeholder='默认 1.12'
          />
          <Input
            label='健康最小样本数'
            type='number'
            name='RoutingHealthMinSamples'
            onChange={handleInputChange}
            value={inputs.RoutingHealthMinSamples}
            min='1'
            max='1000'
            step='1'
            placeholder='默认 5'
          />
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
          <Input
            label='SMTP 服务器地址'
            name='SMTPServer'
            onChange={handleInputChange}
            autoComplete='new-password'
            value={inputs.SMTPServer}
            placeholder='例如：smtp.qq.com'
          />
          <Input
            label='SMTP 端口'
            name='SMTPPort'
            onChange={handleInputChange}
            autoComplete='new-password'
            value={inputs.SMTPPort}
            placeholder='默认: 587'
          />
          <Input
            label='SMTP 账户'
            name='SMTPAccount'
            onChange={handleInputChange}
            autoComplete='new-password'
            value={inputs.SMTPAccount}
            placeholder='通常是邮箱地址'
          />
          <Input
            label='SMTP 访问凭证'
            name='SMTPToken'
            onChange={handleInputChange}
            type='password'
            autoComplete='new-password'
            value={inputs.SMTPToken}
            placeholder='敏感信息'
          />
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
          <Input
            label='GitHub 客户端 ID'
            name='GitHubClientId'
            onChange={handleInputChange}
            autoComplete='new-password'
            value={inputs.GitHubClientId}
            placeholder='输入 ID'
          />
          <Input
            label='GitHub 客户端密钥'
            name='GitHubClientSecret'
            onChange={handleInputChange}
            type='password'
            autoComplete='new-password'
            value={inputs.GitHubClientSecret}
            placeholder='敏感信息'
          />
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
          <Input
            label='Turnstile 站点密钥'
            name='TurnstileSiteKey'
            onChange={handleInputChange}
            autoComplete='new-password'
            value={inputs.TurnstileSiteKey}
            placeholder='输入站点密钥'
          />
          <Input
            label='Turnstile 密钥'
            name='TurnstileSecretKey'
            onChange={handleInputChange}
            type='password'
            autoComplete='new-password'
            value={inputs.TurnstileSecretKey}
            placeholder='敏感信息'
          />
        </div>
        <Button onClick={submitTurnstile} variant="secondary" disabled={loading}>保存 Turnstile 设置</Button>
      </Card>

    </div>
  );
};

export default SystemSetting;
