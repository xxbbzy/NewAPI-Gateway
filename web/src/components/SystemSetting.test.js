import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import SystemSetting from './SystemSetting';
import { API, showError } from '../helpers';

jest.mock('../helpers', () => ({
  API: {
    get: jest.fn(),
    put: jest.fn(),
    post: jest.fn(),
  },
  removeTrailingSlash: (value) => value,
  showError: jest.fn(),
  showSuccess: jest.fn(),
  timestamp2string: (value) => `ts-${value}`,
}));

const flushPromises = async () => {
  await act(async () => {
    await Promise.resolve();
  });
};

global.IS_REACT_ACT_ENVIRONMENT = true;

describe('SystemSetting', () => {
  let container;
  let root;
  let backupRunsData;
  let optionMap;

  const baseOptionMap = {
    PasswordLoginEnabled: 'true',
    PasswordRegisterEnabled: 'true',
    EmailVerificationEnabled: 'false',
    GitHubOAuthEnabled: 'false',
    GitHubClientId: '',
    GitHubClientSecret: '',
    Notice: '',
    SMTPServer: '',
    SMTPPort: '587',
    SMTPAccount: '',
    SMTPToken: '',
    ServerAddress: '',
    Footer: '',
    TurnstileCheckEnabled: 'false',
    TurnstileSiteKey: '',
    TurnstileSecretKey: '',
    RegisterEnabled: 'true',
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
    BackupEnabled: 'true',
    BackupTriggerMode: 'hybrid',
    BackupScheduleCron: '0 */6 * * *',
    BackupMinIntervalSeconds: '600',
    BackupDebounceSeconds: '30',
    BackupWebDAVURL: 'https://dav.example.com/backup',
    BackupWebDAVUsername: 'alice',
    BackupWebDAVPassword: 'secret',
    BackupWebDAVBasePath: '/newapi-gateway-backups',
    BackupEncryptEnabled: 'true',
    BackupEncryptPassphrase: '12345678',
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
  };

  const findButton = (text) => {
    return Array.from(container.querySelectorAll('button')).find((button) => button.textContent.includes(text));
  };

  const setInputValue = async (selector, value) => {
    const input = container.querySelector(selector);
    expect(input).not.toBeNull();
    await act(async () => {
      const previousValue = input.value;
      const valueSetter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set;
      if (valueSetter) {
        valueSetter.call(input, value);
      } else {
        input.value = value;
      }
      const tracker = input._valueTracker;
      if (tracker) {
        tracker.setValue(previousValue);
      }
      input.dispatchEvent(new Event('input', { bubbles: true }));
      input.dispatchEvent(new Event('change', { bubbles: true }));
    });
    await flushPromises();
    await flushPromises();
  };

  beforeEach(() => {
    backupRunsData = [];
    optionMap = { ...baseOptionMap };
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);

    API.get.mockImplementation((url) => {
      if (url === '/api/option/') {
        return Promise.resolve({
          data: {
            success: true,
            data: Object.entries(optionMap).map(([key, value]) => ({ key, value })),
          },
        });
      }
      if (url === '/api/backup/status') {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              pending_retry_count: 0,
              last_run: null,
            },
            preflight: {
              ready: true,
              message: 'ok',
            },
          },
        });
      }
      if (url.startsWith('/api/backup/runs')) {
        return Promise.resolve({
          data: {
            success: true,
            data: backupRunsData,
          },
        });
      }
      if (url.startsWith('/api/backup/retries')) {
        return Promise.resolve({
          data: {
            success: true,
            data: [],
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: [] } });
    });

    API.put.mockResolvedValue({ data: { success: true, message: '' } });
    API.post.mockResolvedValue({ data: { success: true, data: {} } });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    document.body.removeChild(container);
    jest.clearAllMocks();
  });

  it('keeps advanced section collapsed by default and still allows advanced toggle updates', async () => {
    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    const advancedDetails = container.querySelector('details#advanced-settings');
    expect(advancedDetails).not.toBeNull();
    expect(advancedDetails.open).toBe(false);

    await act(async () => {
      advancedDetails.open = true;
      advancedDetails.dispatchEvent(new Event('toggle', { bubbles: true }));
    });

    const checkbox = container.querySelector('input#CheckinScheduleEnabled');
    expect(checkbox).not.toBeNull();

    await act(async () => {
      checkbox.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.put).toHaveBeenCalledWith('/api/option/', {
      key: 'CheckinScheduleEnabled',
      value: 'false',
    });
  });

  it('disables password login toggle when GitHub login is not enabled', async () => {
    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    const passwordToggle = container.querySelector('input#PasswordLoginEnabled');
    expect(passwordToggle).not.toBeNull();
    expect(passwordToggle.disabled).toBe(true);
    expect(container.textContent).toContain('至少保留一种登录方式（密码登录或 GitHub 登录）');
  });

  it('disables GitHub login toggle when password login is not enabled', async () => {
    optionMap.PasswordLoginEnabled = 'false';
    optionMap.GitHubOAuthEnabled = 'true';

    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    const advancedDetails = container.querySelector('details#advanced-settings');
    await act(async () => {
      advancedDetails.open = true;
      advancedDetails.dispatchEvent(new Event('toggle', { bubbles: true }));
    });

    const githubToggle = container.querySelector('input#GitHubOAuthEnabled');
    expect(githubToggle).not.toBeNull();
    expect(githubToggle.disabled).toBe(true);
  });

  it('blocks guarded toggle updates from sending API requests and reports error', async () => {
    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    const passwordToggle = container.querySelector('input#PasswordLoginEnabled');
    expect(passwordToggle).not.toBeNull();
    expect(passwordToggle.disabled).toBe(true);

    API.put.mockClear();
    showError.mockClear();
    await act(async () => {
      passwordToggle.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.put).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalledWith(expect.stringContaining('至少保留一种登录方式'));
  });

  it('allows recovery by enabling password login when GitHub login remains enabled', async () => {
    optionMap.PasswordLoginEnabled = 'false';
    optionMap.GitHubOAuthEnabled = 'true';

    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    const passwordToggle = container.querySelector('input#PasswordLoginEnabled');
    expect(passwordToggle).not.toBeNull();
    expect(passwordToggle.disabled).toBe(false);

    API.put.mockClear();
    await act(async () => {
      passwordToggle.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.put).toHaveBeenCalledWith('/api/option/', {
      key: 'PasswordLoginEnabled',
      value: 'true',
    });
  });

  it('enforces dry-run then confirmation before restore execution', async () => {
    backupRunsData = [
      {
        id: 101,
        status: 'success',
        remote_path: '/newapi-gateway-backups/sqlite-manual-101.zip.enc',
        artifact_path: '',
        started_at: 1700000000,
      },
    ];

    API.post.mockImplementation((url) => {
      if (url === '/api/backup/restore/validate') {
        return Promise.resolve({
          data: {
            success: true,
            data: { ready: true, message: 'dry-run passed' },
          },
        });
      }
      if (url === '/api/backup/restore') {
        return Promise.resolve({
          data: {
            success: true,
            data: {
              message: 'restore completed',
              health: { users: 1, providers: 2, options: 3 },
            },
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: {} } });
    });

    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    let executeButton = findButton('执行恢复');
    expect(executeButton.disabled).toBe(true);

    let dryRunButton = findButton('Dry-Run');
    await act(async () => {
      dryRunButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.post).toHaveBeenCalledWith('/api/backup/restore/validate', {
      run_id: 101,
      dry_run: true,
    });

    await setInputValue('input[name="restoreConfirm"]', 'RESTORE');
    executeButton = findButton('执行恢复');
    expect(executeButton.disabled).toBe(false);

    await act(async () => {
      executeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.post).toHaveBeenCalledWith('/api/backup/restore', {
      run_id: 101,
      dry_run: false,
      confirm: true,
    });
  });

  it('falls back to manual restore path when no automatic candidate exists', async () => {
    backupRunsData = [
      {
        id: 201,
        status: 'failed',
        remote_path: '/newapi-gateway-backups/failed.zip.enc',
        artifact_path: '',
      },
    ];

    API.post.mockImplementation((url, payload) => {
      if (url === '/api/backup/restore/validate') {
        return Promise.resolve({
          data: {
            success: true,
            data: { ready: true, message: 'dry-run passed' },
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: payload || {} } });
    });

    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    let dryRunButton = findButton('Dry-Run');
    await act(async () => {
      dryRunButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(expect.stringContaining('未找到可用恢复候选'));

    const advancedDetails = container.querySelector('details#advanced-settings');
    await act(async () => {
      advancedDetails.open = true;
      advancedDetails.dispatchEvent(new Event('toggle', { bubbles: true }));
    });

    await setInputValue('input[name="restorePath"]', '/tmp/manual-backup.zip.enc');

    dryRunButton = findButton('Dry-Run');
    await act(async () => {
      dryRunButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.post).toHaveBeenCalledWith('/api/backup/restore/validate', {
      local_path: '/tmp/manual-backup.zip.enc',
      dry_run: true,
    });
  });

  it('blocks saving when backup is enabled but WebDAV URL is empty', async () => {
    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    await setInputValue('input[name="BackupWebDAVURL"]', '');

    const saveButton = findButton('保存备份设置');
    await act(async () => {
      saveButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(expect.stringContaining('已启用备份，但未填写 WebDAV URL'));
    expect(API.put).not.toHaveBeenCalledWith('/api/option/', expect.objectContaining({ key: 'BackupWebDAVURL' }));
  });

  it('shows restore error summary when execution fails', async () => {
    backupRunsData = [
      {
        id: 410,
        status: 'success',
        remote_path: '/newapi-gateway-backups/sqlite-manual-410.zip.enc',
        artifact_path: '',
      },
    ];

    API.post.mockImplementation((url) => {
      if (url === '/api/backup/restore/validate') {
        return Promise.resolve({
          data: {
            success: true,
            data: { ready: true, message: 'dry-run passed' },
          },
        });
      }
      if (url === '/api/backup/restore') {
        return Promise.resolve({
          data: {
            success: false,
            message: 'restore failed by checksum',
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: {} } });
    });

    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    const dryRunButton = findButton('Dry-Run');
    await act(async () => {
      dryRunButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    await setInputValue('input[name="restoreConfirm"]', 'RESTORE');
    const executeButton = findButton('执行恢复');
    await act(async () => {
      executeButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(showError).toHaveBeenCalledWith('restore failed by checksum');
    expect(container.textContent).toContain('恢复流程出现错误');
    expect(container.textContent).toContain('restore failed by checksum');
  });

  it('prefers success run with remote_path over artifact-only candidate', async () => {
    backupRunsData = [
      {
        id: 301,
        status: 'success',
        remote_path: '',
        artifact_path: '/tmp/local-artifact.zip.enc',
      },
      {
        id: 302,
        status: 'success',
        remote_path: '/newapi-gateway-backups/remote-artifact.zip.enc',
        artifact_path: '',
      },
    ];

    API.post.mockImplementation((url) => {
      if (url === '/api/backup/restore/validate') {
        return Promise.resolve({
          data: {
            success: true,
            data: { ready: true, message: 'dry-run passed' },
          },
        });
      }
      return Promise.resolve({ data: { success: true, data: {} } });
    });

    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

    const dryRunButton = findButton('Dry-Run');
    await act(async () => {
      dryRunButton.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    await flushPromises();

    expect(API.post).toHaveBeenCalledWith('/api/backup/restore/validate', {
      run_id: 302,
      dry_run: true,
    });
  });
});
