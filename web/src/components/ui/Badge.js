import React from 'react';

const Badge = ({ children, color = 'gray' }) => {
    const colors = {
        gray: { bg: 'var(--gray-100)', text: 'var(--gray-800)' },
        green: { bg: 'rgba(16, 185, 129, 0.1)', text: 'var(--success)' },
        red: { bg: 'rgba(239, 68, 68, 0.1)', text: 'var(--error)' },
        blue: { bg: 'rgba(59, 130, 246, 0.1)', text: 'var(--info)' },
        yellow: { bg: 'rgba(245, 158, 11, 0.1)', text: 'var(--warning)' },
        orange: { bg: 'rgba(245, 158, 11, 0.12)', text: 'var(--warning-color)' },
        purple: { bg: 'rgba(139, 92, 246, 0.14)', text: '#8b5cf6' },
    };

    const style = colors[color] || colors.gray;

    return (
        <span
            style={{
                display: 'inline-flex',
                alignItems: 'center',
                padding: '0.125rem 0.625rem',
                borderRadius: '9999px',
                fontSize: '0.75rem',
                fontWeight: '500',
                backgroundColor: style.bg,
                color: style.text,
            }}
        >
            {children}
        </span>
    );
};

export default Badge;
