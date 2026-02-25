import React from 'react';
import Button from './Button';
import { ChevronLeft, ChevronRight } from 'lucide-react';

const Pagination = ({ activePage, totalPages, onPageChange }) => {
    const safeTotalPages = Number.isFinite(Number(totalPages)) ? Number(totalPages) : 0;
    const safeActivePage = Number.isFinite(Number(activePage)) ? Number(activePage) : 1;
    if (safeTotalPages <= 1) return null;

    const pages = [];
    const windowSize = 2;
    let start = Math.max(1, safeActivePage - windowSize);
    let end = Math.min(safeTotalPages, safeActivePage + windowSize);

    if (start > 1) {
        pages.push(1);
        if (start > 2) pages.push('...');
    }

    for (let i = start; i <= end; i++) {
        pages.push(i);
    }

    if (end < safeTotalPages) {
        if (end < safeTotalPages - 1) pages.push('...');
        pages.push(safeTotalPages);
    }

    return (
        <div className='table-pagination'>
            <Button
                variant="secondary"
                size="sm"
                aria-label='上一页'
                disabled={safeActivePage === 1}
                onClick={() => onPageChange(null, { activePage: safeActivePage - 1 })}
                icon={ChevronLeft}
            />

            {pages.map((p, idx) => (
                <React.Fragment key={idx}>
                    {p === '...' ? (
                        <span className='pagination-ellipsis'>...</span>
                    ) : (
                        <Button
                            variant={p === safeActivePage ? 'primary' : 'secondary'}
                            size="sm"
                            aria-label={`第 ${p} 页`}
                            onClick={() => onPageChange(null, { activePage: p })}
                        >
                            {p}
                        </Button>
                    )}
                </React.Fragment>
            ))}

            <Button
                variant="secondary"
                size="sm"
                aria-label='下一页'
                disabled={safeActivePage === safeTotalPages}
                onClick={() => onPageChange(null, { activePage: safeActivePage + 1 })}
                icon={ChevronRight}
            />
        </div>
    );
};

export default Pagination;
