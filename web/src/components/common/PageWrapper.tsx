'use client';

import { ReactNode } from 'react';

interface PageWrapperProps {
  children: ReactNode;
  className?: string;
  childLayout?: boolean;
}

export function PageWrapper({ children, className = 'space-y-6' }: PageWrapperProps) {
  return (
    <div className={`min-h-0 ${className}`}>
      {children}
    </div>
  );
}
