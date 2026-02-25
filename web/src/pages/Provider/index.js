import React from 'react';
import ProvidersTable from '../../components/ProvidersTable';

const Provider = () => (
    <div className='page-shell'>
        <div className='page-header'>
            <h2 className='page-title'>供应商管理</h2>
        </div>
        <ProvidersTable />
    </div>
);

export default Provider;
