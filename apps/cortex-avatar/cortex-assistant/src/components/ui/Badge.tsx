import React from 'react';
import { clsx } from 'clsx';

interface BadgeProps {
  variant?: 'default' | 'primary' | 'success' | 'warning' | 'danger';
  size?: 'sm' | 'md';
  children: React.ReactNode;
  className?: string;
}

export function Badge({
  variant = 'default',
  size = 'sm',
  children,
  className,
}: BadgeProps) {
  const variantClasses = {
    default: 'bg-gray-100 text-gray-800 dark:bg-surface-700 dark:text-gray-300',
    primary: 'badge-primary',
    success: 'badge-success',
    warning: 'badge-warning',
    danger: 'badge-danger',
  };

  const sizeClasses = {
    sm: 'px-2 py-0.5 text-xs',
    md: 'px-2.5 py-1 text-sm',
  };

  return (
    <span
      className={clsx(
        'inline-flex items-center font-medium rounded-full',
        variantClasses[variant],
        sizeClasses[size],
        className
      )}
    >
      {children}
    </span>
  );
}
