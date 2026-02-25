import React from 'react';
import UsersTable from '../../components/UsersTable';

const User = () => (
  <div className='page-shell'>
    <div className='page-header'>
      <h2 className='page-title'>管理用户</h2>
    </div>
    <UsersTable />
  </div>
);

export default User;
