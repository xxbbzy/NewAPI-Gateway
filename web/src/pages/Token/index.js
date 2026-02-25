import React from 'react';
import AggTokensTable from '../../components/AggTokensTable';

const Token = () => (
    <div className='page-shell'>
        <div className='page-header'>
            <h2 className='page-title'>聚合令牌管理</h2>
        </div>
        <AggTokensTable />
    </div>
);

export default Token;
