import React from 'react';

const Card = ({ children, className = '', title, padding = '1.5rem', style, titleStyle, bodyStyle }) => {
    return (
        <div
            style={{
                backgroundColor: 'var(--bg-primary)',
                borderRadius: 'var(--radius-lg)',
                border: '1px solid var(--border-color)',
                boxShadow: 'var(--shadow-sm)',
                overflow: 'hidden',
                marginBottom: '1rem',
                ...style,
            }}
            className={className}
        >
            {title && (
                <div
                    style={{
                        padding: '1rem 1.5rem',
                        borderBottom: '1px solid var(--border-color)',
                        fontWeight: '600',
                        fontSize: '1.125rem',
                        color: 'var(--text-primary)',
                        ...titleStyle,
                    }}
                >
                    {title}
                </div>
            )}
            <div style={{ padding, ...bodyStyle }}>
                {children}
            </div>
        </div>
    );
};

export default Card;
