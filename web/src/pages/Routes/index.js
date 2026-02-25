import React from 'react';
import ModelRoutesTable from '../../components/ModelRoutesTable';

const Routes = () => (
    <div className='page-shell routes-page'>
        <div className='page-header'>
            <h2 className='page-title routes-page-title'>模型路由</h2>
        </div>
        <div className='routes-page-body'>
            <ModelRoutesTable />
        </div>
    </div>
);

export default Routes;
