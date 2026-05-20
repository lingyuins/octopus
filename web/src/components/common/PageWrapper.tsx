'use client';

import { motion, AnimatePresence } from 'motion/react';
import { ReactNode, isValidElement, Children } from 'react';

interface PageWrapperProps {
  children: ReactNode;
  className?: string;
}

export function PageWrapper({ children, className = 'space-y-6' }: PageWrapperProps) {
  const childArray = Children.toArray(children);

  return (
    <motion.div className={`min-h-0 ${className}`}>
      <AnimatePresence>
        {childArray.map((child, index) => {
          const key = isValidElement(child) ? child.key : null;

          return (
            <motion.div
              key={key ?? index}
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{
                duration: 0.2,
                delay: Math.min(index * 0.03, 0.2),
              }}
            >
              {child}
            </motion.div>
          );
        })}
      </AnimatePresence>
    </motion.div>
  );
}
