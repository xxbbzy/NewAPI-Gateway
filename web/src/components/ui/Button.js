import React from 'react';

const Button = ({
    children,
    variant = 'primary', // primary, secondary, outline, danger, ghost
    size = 'md', // sm, md, lg
    className = '',
    loading = false,
    disabled = false,
    onClick,
    type = 'button',
    icon: Icon,
    ...rest
}) => {
    const baseStyles = {
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontWeight: '500',
        borderRadius: 'var(--radius-md)',
        transition: 'all 0.2s',
        cursor: disabled || loading ? 'not-allowed' : 'pointer',
        opacity: disabled || loading ? 0.6 : 1,
        border: '1px solid transparent',
        outline: 'none',
    };

    const variants = {
        primary: {
            backgroundColor: 'var(--primary-600)',
            color: 'white',
            border: '1px solid var(--primary-600)',
        },
        secondary: {
            backgroundColor: 'var(--bg-primary)',
            color: 'var(--gray-700)',
            border: '1px solid var(--gray-300)',
        },
        outline: {
            backgroundColor: 'transparent',
            color: 'var(--primary-600)',
            border: '1px solid var(--primary-600)',
        },
        danger: {
            backgroundColor: 'var(--error)',
            color: 'white',
            border: '1px solid var(--error)',
        },
        ghost: {
            backgroundColor: 'transparent',
            color: 'var(--gray-600)',
        }
    };

    const sizes = {
        sm: { padding: '0.25rem 0.5rem', fontSize: '0.875rem' },
        md: { padding: '0.5rem 1rem', fontSize: '1rem' },
        lg: { padding: '0.75rem 1.5rem', fontSize: '1.125rem' },
    };

    const style = {
        ...baseStyles,
        ...variants[variant],
        ...sizes[size],
    };

    return (
        <button
            {...rest}
            type={type}
            style={style}
            className={className}
            disabled={disabled || loading}
            onClick={onClick}
            onMouseEnter={(e) => {
                if (!disabled && !loading) {
                    if (variant === 'primary') e.target.style.backgroundColor = 'var(--primary-700)';
                    if (variant === 'secondary') e.target.style.backgroundColor = 'var(--gray-50)';
                    if (variant === 'ghost') e.target.style.backgroundColor = 'var(--gray-100)';
                }
            }}
            onMouseLeave={(e) => {
                if (!disabled && !loading) {
                    e.target.style.backgroundColor = variants[variant].backgroundColor;
                }
            }}
        >
            {loading && (
                <svg className="animate-spin" style={{ marginRight: '0.5rem', height: '1em', width: '1em' }} xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
            )}
            {!loading && Icon && <Icon size={16} style={{ marginRight: children ? '0.5rem' : 0 }} />}
            {children}
        </button>
    );
};

export default Button;
