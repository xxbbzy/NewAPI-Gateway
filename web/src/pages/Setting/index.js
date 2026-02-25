import React from 'react';
import SystemSetting from '../../components/SystemSetting';
import { isRoot } from '../../helpers';
import OtherSetting from '../../components/OtherSetting';
import PersonalSetting from '../../components/PersonalSetting';
import Tabs from '../../components/ui/Tabs';

const Setting = () => {
  let tabs = [
    {
      label: '个人设置',
      content: <PersonalSetting />
    }
  ];

  if (isRoot()) {
    tabs.push({
      label: '系统设置',
      content: <SystemSetting />
    });
    tabs.push({
      label: '其他设置',
      content: <OtherSetting />
    });
  }

  return (
    <div className='page-shell'>
      <div className='page-header'>
        <h2 className='page-title'>设置</h2>
      </div>
      <div className='page-section page-section--tight'>
        <Tabs items={tabs} />
      </div>
    </div>
  );
};

export default Setting;
