import React from 'react';
import LogsTable from '../../components/LogsTable';

const Log = () => (
    <div className='page-shell'>
        <div className='page-header'>
            <h2 className='page-title'>日志查询</h2>
        </div>
        <LogsTable selfOnly={false} />
    </div>
);

export default Log;
