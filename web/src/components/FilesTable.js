import React, { useEffect, useState } from 'react';
import { API, copy, normalizePaginatedData, showError, showSuccess } from '../helpers';
import { useDropzone } from 'react-dropzone';
import { ITEMS_PER_PAGE } from '../constants';
import { Table, Thead, Tbody, Tr, Th, Td } from './ui/Table';
import Button from './ui/Button';
import Input from './ui/Input';
import ProgressBar from './ui/ProgressBar';
import Pagination from './ui/Pagination';
import { UploadCloud, Search, Download, Trash2, Copy } from 'lucide-react';

const FilesTable = () => {
  const [files, setFiles] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [totalPages, setTotalPages] = useState(0);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [searchMode, setSearchMode] = useState(false);
  const { acceptedFiles, getRootProps, getInputProps } = useDropzone();
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);

  const loadFiles = async (page) => {
    setLoading(true);
    try {
      const res = await API.get(`/api/file/?p=${page}&page_size=${ITEMS_PER_PAGE}`);
      const { success, message, data } = res.data;
      if (success) {
        const normalized = normalizePaginatedData(data, { p: page, page_size: ITEMS_PER_PAGE });
        setFiles(Array.isArray(normalized.items) ? normalized.items : []);
        setTotalPages(Number(normalized.total_pages || 0));
        setSearchMode(false);
      } else {
        showError(message);
      }
    } catch (error) {
      showError('加载文件失败');
    } finally {
      setLoading(false);
    }
  };

  const onPaginationChange = (e, { activePage }) => {
    if (searchMode) return;
    if (activePage < 1) return;
    const effectiveTotalPages = Math.max(totalPages, 1);
    if (activePage > effectiveTotalPages) return;
    setActivePage(activePage);
  };

  useEffect(() => {
    if (searchMode) return;
    loadFiles(activePage - 1).catch((reason) => {
      showError(reason);
    });
  }, [activePage, searchMode]);

  const downloadFile = (link, filename) => {
    let linkElement = document.createElement('a');
    linkElement.download = filename;
    linkElement.href = '/upload/' + link;
    linkElement.click();
  };

  const copyLink = (link) => {
    let url = window.location.origin + '/upload/' + link;
    copy(url).then();
    showSuccess('链接已复制到剪贴板');
  };

  const deleteFile = async (id, idx) => {
    const res = await API.delete(`/api/file/${id}`);
    const { success, message } = res.data;
    if (success) {
      let newFiles = [...files];
      const realIdx = idx;
      newFiles[realIdx].deleted = true;
      setFiles(newFiles);
      showSuccess('文件已删除！');
    } else {
      showError(message);
    }
  };

  const searchFiles = async (e) => {
    if (e) e.preventDefault();
    if (searchKeyword === '') {
      await loadFiles(0);
      setActivePage(1);
      return;
    }
    setSearching(true);
    const res = await API.get(`/api/file/search?keyword=${searchKeyword}`);
    const { success, message, data } = res.data;
    if (success) {
      setFiles(Array.isArray(data) ? data : []);
      setActivePage(1);
      setTotalPages(1);
      setSearchMode(true);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const handleKeywordChange = async (e) => {
    setSearchKeyword(e.target.value.trim());
  };

  const sortFile = (key) => {
    if (files.length === 0) return;
    setLoading(true);
    let sortedUsers = [...files];
    sortedUsers.sort((a, b) => {
      return ('' + a[key]).localeCompare(b[key]);
    });
    if (sortedUsers[0].id === files[0].id) {
      sortedUsers.reverse();
    }
    setFiles(sortedUsers);
    setLoading(false);
  };

  const uploadFiles = async () => {
    if (acceptedFiles.length === 0) return;
    setUploading(true);
    let formData = new FormData();
    for (let i = 0; i < acceptedFiles.length; i++) {
      formData.append('file', acceptedFiles[i]);
    }
    const res = await API.post(`/api/file`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data',
      },
      onUploadProgress: (e) => {
        let uploadProgress = ((e.loaded / e.total) * 100).toFixed(2);
        setUploadProgress(uploadProgress);
      },
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(`${acceptedFiles.length} 个文件上传成功！`);
    } else {
      showError(message);
    }
    setUploading(false);
    setUploadProgress(0);
    setSearchKeyword('');
    setSearchMode(false);
    loadFiles(0).then();
    setActivePage(1);
  };

  useEffect(() => {
    uploadFiles().then();
  }, [acceptedFiles]);

  const visibleFiles = files.filter((file) => !file.deleted);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
      <div
        {...getRootProps({ className: 'dropzone' })}
        style={{
          border: '2px dashed var(--border-color)',
          borderRadius: 'var(--radius-lg)',
          padding: '2rem',
          textAlign: 'center',
          backgroundColor: 'var(--gray-50)',
          cursor: 'pointer',
          transition: 'all 0.2s'
        }}
      >
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.5rem' }}>
          <UploadCloud size={40} color="var(--text-secondary)" />
          <span style={{ fontSize: '1rem', fontWeight: '500', color: 'var(--text-primary)' }}>拖拽上传或点击上传</span>
          <input {...getInputProps()} />
        </div>
      </div>

      {uploading && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.875rem' }}>
            <span>上传中...</span>
            <span>{uploadProgress}%</span>
          </div>
          <ProgressBar percent={uploadProgress} />
        </div>
      )}

      <form onSubmit={searchFiles}>
        <Input
          icon={Search}
          placeholder='搜索文件的名称，上传者以及描述信息 ...'
          value={searchKeyword}
          onChange={handleKeywordChange}
        />
      </form>

      <div style={{ backgroundColor: 'var(--bg-primary)', borderRadius: 'var(--radius-lg)', border: '1px solid var(--border-color)', overflow: 'hidden' }}>
        <Table>
          <Thead>
            <Tr>
              <Th onClick={() => sortFile('filename')} style={{ cursor: 'pointer' }}>文件名</Th>
              <Th onClick={() => sortFile('uploader_id')} style={{ cursor: 'pointer' }}>上传者</Th>
              <Th onClick={() => sortFile('email')} style={{ cursor: 'pointer' }}>上传时间</Th>
              <Th>操作</Th>
            </Tr>
          </Thead>
          <Tbody>
            {visibleFiles
              .map((file, idx) => {
                return (
                  <Tr key={file.id}>
                    <Td>
                      <a href={'/upload/' + file.link} target='_blank' rel="noreferrer" style={{ color: 'var(--primary-600)', textDecoration: 'none' }}>
                        {file.filename}
                      </a>
                    </Td>
                    <Td title={'上传者编号：' + file.uploader_id}>{file.uploader}</Td>
                    <Td>{file.upload_time}</Td>
                    <Td>
                      <div style={{ display: 'flex', gap: '0.5rem' }}>
                        <Button
                          size="sm"
                          title="下载"
                          onClick={() => downloadFile(file.link, file.filename)}
                        >
                          <Download size={16} />
                        </Button>
                        <Button
                          size="sm"
                          variant="secondary"
                          title="复制链接"
                          onClick={() => copyLink(file.link)}
                        >
                          <Copy size={16} />
                        </Button>
                        <Button
                          size="sm"
                          variant="danger"
                          title="删除"
                          onClick={() => deleteFile(file.id, idx)}
                        >
                          <Trash2 size={16} />
                        </Button>
                      </div>
                    </Td>
                  </Tr>
                );
              })}
          </Tbody>
        </Table>
        <div style={{ padding: '1rem', borderTop: '1px solid var(--border-color)' }}>
          <Pagination
            activePage={activePage}
            totalPages={Math.max(searchMode ? 1 : totalPages, 1)}
            onPageChange={onPaginationChange}
          />
        </div>
      </div>
    </div>
  );
};

export default FilesTable;
