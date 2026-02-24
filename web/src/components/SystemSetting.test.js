import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import SystemSetting from './SystemSetting';
import { API } from '../helpers';

jest.mock('../helpers', () => ({
  API: {
    get: jest.fn(),
    put: jest.fn(),
  },
  removeTrailingSlash: (value) => value,
  showError: jest.fn(),
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

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);

    const optionMap = {
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
    };

    API.get.mockResolvedValue({
      data: {
        success: true,
        data: Object.entries(optionMap).map(([key, value]) => ({ key, value })),
      },
    });
    API.put.mockResolvedValue({ data: { success: true, message: '' } });
  });

  afterEach(async () => {
    await act(async () => {
      root.unmount();
    });
    document.body.removeChild(container);
    jest.clearAllMocks();
  });

  it('toggles CheckinScheduleEnabled with true/false payload', async () => {
    await act(async () => {
      root.render(<SystemSetting />);
    });
    await flushPromises();
    await flushPromises();

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
});
