import React from 'react';
import { clsx } from 'clsx';
import { Loader2 } from 'lucide-react';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean;
  icon?: React.ReactNode;
  iconPosition?: 'left' | 'right';
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = 'primary',
      size = 'md',
      loading = false,
      icon,
      iconPosition = 'left',
      disabled,
      children,
      ...props
    },
    ref
  ) => {
    const variantClasses = {
      primary: 'btn-primary',
      secondary: 'btn-secondary',
      ghost: 'btn-ghost',
      danger: 'btn-danger',
    };

    const sizeClasses = {
      sm: 'px-3 py-1.5 text-xs',
      md: 'px-4 py-2 text-sm',
      lg: 'px-6 py-3 text-base',
    };

    const iconOnly = !children && icon;

    return (
      <button
        ref={ref}
        className={clsx(
          variantClasses[variant],
          sizeClasses[size],
          iconOnly && 'btn-icon',
          className
        )}
        disabled={disabled || loading}
        {...props}
      >
        {loading ? (
          <Loader2 className="w-4 h-4 animate-spin" />
        ) : (
          <>
            {icon && iconPosition === 'left' && <span className="shrink-0">{icon}</span>}
            {children}
            {icon && iconPosition === 'right' && <span className="shrink-0">{icon}</span>}
          </>
        )}
      </button>
    );
  }
);

Button.displayName = 'Button';
