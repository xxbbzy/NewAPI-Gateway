import React from 'react';
import FilesTable from '../../components/FilesTable';

const File = () => (
  <div className='page-shell'>
    <div className='page-header'>
      <h2 className='page-title'>管理文件</h2>
    </div>
    <FilesTable />
  </div>
);

export default File;
