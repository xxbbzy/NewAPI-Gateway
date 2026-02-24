import React, { useEffect } from 'react';
import { X } from 'lucide-react';

const Modal = ({ isOpen, onClose, title, children, actions, closeOnOverlayClick = true }) => {
    useEffect(() => {
        if (isOpen) {
            document.body.style.overflow = 'hidden';
        } else {
            document.body.style.overflow = 'unset';
        }
        return () => {
            document.body.style.overflow = 'unset';
        };
    }, [isOpen]);

    if (!isOpen) return null;

    return (
        <div
            style={{
                position: 'fixed',
                top: 0,
                left: 0,
                width: '100vw',
                height: '100vh',
                backgroundColor: 'rgba(0, 0, 0, 0.5)',
                zIndex: 50,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                padding: '1rem',
            }}
            onClick={closeOnOverlayClick ? onClose : undefined}
        >
            <div
                style={{
                    backgroundColor: 'var(--bg-primary)',
                    borderRadius: 'var(--radius-lg)',
                    boxShadow: 'var(--shadow-lg)',
                    width: '100%',
                    maxWidth: '32rem', // max-w-lg
                    maxHeight: '90vh',
                    display: 'flex',
                    flexDirection: 'column',
                    position: 'relative',
                }}
                onClick={(e) => e.stopPropagation()}
            >
                <div
                    style={{
                        padding: '1.25rem',
                        borderBottom: '1px solid var(--border-color)',
                        display: 'flex',
                        justifyContent: 'space-between',
                        alignItems: 'center',
                    }}
                >
                    <h3 style={{ fontSize: '1.125rem', fontWeight: '600' }}>{title}</h3>
                    <button
                        onClick={onClose}
                        style={{
                            background: 'none',
                            border: 'none',
                            cursor: 'pointer',
                            color: 'var(--gray-400)',
                            padding: '0.25rem',
                        }}
                    >
                        <X size={20} />
                    </button>
                </div>

                <div style={{ padding: '1.5rem', overflowY: 'auto' }}>
                    {children}
                </div>

                {actions && (
                    <div
                        style={{
                            padding: '1rem 1.5rem',
                            backgroundColor: 'var(--gray-50)',
                            borderTop: '1px solid var(--border-color)',
                            display: 'flex',
                            justifyContent: 'flex-end',
                            gap: '0.75rem',
                            borderBottomLeftRadius: 'var(--radius-lg)',
                            borderBottomRightRadius: 'var(--radius-lg)',
                        }}
                    >
                        {actions}
                    </div>
                )}
            </div>
        </div>
    );
};

export default Modal;
