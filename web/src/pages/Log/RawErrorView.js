import React, { useEffect, useMemo, useState } from 'react';

const RawErrorView = () => {
  const key = useMemo(() => {
    const params = new URLSearchParams(window.location.search);
    return params.get('key') || '';
  }, []);
  const [content, setContent] = useState('');
  const [requestId, setRequestId] = useState('');

  useEffect(() => {
    if (!key) {
      setContent('缺少 key 参数');
      return;
    }
    const raw = localStorage.getItem(key);
    if (!raw) {
      setContent('未找到对应错误详情，可能已过期或跨域打开导致不可见。');
      return;
    }
    try {
      const parsed = JSON.parse(raw);
      setRequestId(parsed?.request_id || '');
      setContent(JSON.stringify(parsed, null, 2));
    } catch (e) {
      setContent(raw);
    }
  }, [key]);

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg-secondary)', color: 'var(--text-primary)', padding: '20px' }}>
      <div style={{ marginBottom: '12px', fontSize: '13px', color: 'var(--text-secondary)' }}>
        request_id: {requestId || '-'} | key: {key || '-'}
      </div>
      <pre
        style={{
          margin: 0,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
          lineHeight: 1.45,
          background: 'var(--bg-primary)',
          border: '1px solid var(--border-color)',
          color: 'var(--text-primary)',
          borderRadius: '8px',
          padding: '16px',
          fontSize: '13px',
          fontFamily:
            'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace'
        }}
      >
        {content}
      </pre>
    </div>
  );
};

export default RawErrorView;
