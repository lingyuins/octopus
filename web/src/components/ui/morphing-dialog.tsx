'use client';

import React, {
  useCallback,
  useContext,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  motion,
  AnimatePresence,
  MotionConfig,
  Transition,
  Variant,
} from 'motion/react';
import { createPortal } from 'react-dom';
import { useTranslations } from 'next-intl';
import { cn } from '@/lib/utils';
import { XIcon } from 'lucide-react';
import useClickOutside from '@/hooks/useClickOutside';
import { getMorphingDialogLifecycleEvent } from './morphing-dialog-state';

export type MorphingDialogContextType = {
  isOpen: boolean;
  setIsOpen: React.Dispatch<React.SetStateAction<boolean>>;
  uniqueId: string;
  disableSharedLayout?: boolean;
  triggerRef: React.RefObject<HTMLDivElement | null>;
  onOpen?: () => void;
  onClose?: () => void;
};

const MorphingDialogContext =
  React.createContext<MorphingDialogContextType | null>(null);

function useMorphingDialog() {
  const context = useContext(MorphingDialogContext);
  if (!context) {
    throw new Error(
      'useMorphingDialog must be used within a MorphingDialogProvider'
    );
  }
  return context;
}

export type MorphingDialogProviderProps = {
  children: React.ReactNode;
  transition?: Transition;
  disableSharedLayout?: boolean;
  onOpen?: () => void;
  onClose?: () => void;
};

function MorphingDialogProvider({
  children,
  transition,
  disableSharedLayout,
  onOpen,
  onClose,
}: MorphingDialogProviderProps) {
  const [isOpen, setIsOpen] = useState(false);
  const uniqueId = useId();
  const triggerRef = useRef<HTMLDivElement>(null!);
  const pendingLifecycleEventRef = useRef<ReturnType<typeof getMorphingDialogLifecycleEvent>>(null);

  useEffect(() => {
    const event = pendingLifecycleEventRef.current;
    if (event === 'opened') {
      onOpen?.();
    } else if (event === 'closed') {
      onClose?.();
    }
    pendingLifecycleEventRef.current = null;
  }, [isOpen, onOpen, onClose]);

  const contextValue = useMemo(
    () => ({
      isOpen,
      setIsOpen: (value: React.SetStateAction<boolean>) => {
        setIsOpen((prev) => {
          const next = typeof value === 'function' ? value(prev) : value;
          pendingLifecycleEventRef.current = getMorphingDialogLifecycleEvent(prev, next);
          return next;
        });
      },
      uniqueId,
      disableSharedLayout,
      triggerRef,
      onOpen,
      onClose,
    }),
    [isOpen, uniqueId, disableSharedLayout, onOpen, onClose]
  );

  return (
    <MorphingDialogContext.Provider value={contextValue}>
      <MotionConfig transition={transition} reducedMotion='user'>
        {children}
      </MotionConfig>
    </MorphingDialogContext.Provider>
  );
}

export type MorphingDialogProps = {
  children: React.ReactNode;
  transition?: Transition;
  disableSharedLayout?: boolean;
  onOpen?: () => void;
  onClose?: () => void;
};

function MorphingDialog({ children, transition, disableSharedLayout, onOpen, onClose }: MorphingDialogProps) {
  return (
    <MorphingDialogProvider
      transition={transition}
      disableSharedLayout={disableSharedLayout}
      onOpen={onOpen}
      onClose={onClose}
    >
      {children}
    </MorphingDialogProvider>
  );
}

export type MorphingDialogTriggerProps = {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  triggerRef?: React.RefObject<HTMLDivElement>;
  ariaLabel?: string;
  disabled?: boolean;
};

function MorphingDialogTrigger({
  children,
  className,
  style,
  triggerRef: triggerRefProp,
  ariaLabel,
  disabled,
}: MorphingDialogTriggerProps) {
  const t = useTranslations('common.dialog');
  const { setIsOpen, isOpen, uniqueId, disableSharedLayout, triggerRef } = useMorphingDialog();

  const handleClick = useCallback(() => {
    if (disabled) return;
    setIsOpen(!isOpen);
  }, [disabled, isOpen, setIsOpen]);

  const handleKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      if (disabled) return;
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        setIsOpen(!isOpen);
      }
    },
    [disabled, isOpen, setIsOpen]
  );

  // Important: when dialog is open, framer-motion shared-layout can temporarily
  // "flash" the trigger back into its original position during internal re-layouts.
  // To make this robust, we render a non-motion placeholder (still in layout flow)
  // instead of the motion trigger while open.
  if (isOpen) {
    return (
      <div
        ref={triggerRefProp ?? triggerRef}
        className={cn('relative', className)}
        style={{ ...style, visibility: 'hidden', pointerEvents: 'none' }}
        aria-hidden
      >
        {children}
      </div>
    );
  }

  return (
    <motion.div
      ref={triggerRefProp ?? triggerRef}
      layoutId={disableSharedLayout ? undefined : `dialog-${uniqueId}`}
      className={cn('relative cursor-pointer', disabled && 'cursor-not-allowed opacity-50', className)}
      onClick={handleClick}
      onKeyDown={handleKeyDown}
      style={style}
      aria-haspopup='dialog'
      aria-expanded={isOpen}
      aria-disabled={disabled || undefined}
      aria-controls={`motion-ui-morphing-dialog-content-${uniqueId}`}
      aria-label={ariaLabel ?? t('open')}
      role='button'
      tabIndex={disabled ? -1 : 0}
    >
      {children}
    </motion.div>
  );
}

export type MorphingDialogContentProps = {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
};

function MorphingDialogContent({
  children,
  className,
  style,
}: MorphingDialogContentProps) {
  const { setIsOpen, isOpen, uniqueId, disableSharedLayout, triggerRef } = useMorphingDialog();
  const containerRef = useRef<HTMLDivElement>(null!);
  const firstFocusableElementRef = useRef<HTMLElement | null>(null);
  const lastFocusableElementRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
      if (event.key === 'Tab') {
        if (!firstFocusableElementRef.current || !lastFocusableElementRef.current) return;

        if (event.shiftKey) {
          if (document.activeElement === firstFocusableElementRef.current) {
            event.preventDefault();
            lastFocusableElementRef.current.focus();
          }
        } else {
          if (document.activeElement === lastFocusableElementRef.current) {
            event.preventDefault();
            firstFocusableElementRef.current.focus();
          }
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);

    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [setIsOpen]);

  useEffect(() => {
    if (isOpen) {
      document.body.classList.add('overflow-hidden');
      const focusableElements = containerRef.current?.querySelectorAll(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusableElements && focusableElements.length > 0) {
        firstFocusableElementRef.current = focusableElements[0] as HTMLElement;
        lastFocusableElementRef.current = focusableElements[focusableElements.length - 1] as HTMLElement;
        (focusableElements[0] as HTMLElement).focus();
      }
    } else {
      document.body.classList.remove('overflow-hidden');
      triggerRef.current?.focus();
    }

    return () => {
      document.body.classList.remove('overflow-hidden');
    };
  }, [isOpen, triggerRef]);

  useClickOutside(
    containerRef,
    () => {
      if (isOpen) {
        setIsOpen(false);
      }
    },
    (event) => {
      const target = event.target as HTMLElement | null;
      if (target?.closest('[data-slot="select-content"]')) {
        return true;
      }
      const openSelectContent = document.querySelector('[data-slot="select-content"]');
      if (openSelectContent) {
        return true;
      }
      if (target?.closest('[data-slot="popover-content"]')) {
        return true;
      }
      const openPopoverContent = document.querySelector('[data-slot="popover-content"]');
      if (openPopoverContent) {
        return true;
      }
      const dialogLayer = target?.closest('[data-slot="morphing-dialog-layer"]') as HTMLElement | null;
      if (dialogLayer && dialogLayer.dataset.dialogId !== uniqueId) {
        return true;
      }
      return false;
    }
  );

  return (
    <motion.div
      ref={containerRef}
      layoutId={disableSharedLayout ? undefined : `dialog-${uniqueId}`}
      initial={disableSharedLayout ? { opacity: 0, scale: 0.98 } : undefined}
      animate={disableSharedLayout ? { opacity: 1, scale: 1 } : undefined}
      exit={disableSharedLayout ? { opacity: 0, scale: 0.98 } : undefined}
      transition={disableSharedLayout ? { duration: 0.12, ease: 'easeOut' } : undefined}
      className={cn(
        'relative flex flex-col overflow-hidden rounded-xl border border-border bg-card shadow-lg w-[calc(100vw-2rem)] sm:max-w-2xl max-h-[85dvh] sm:max-h-[90dvh]',
        className
      )}
      data-slot='morphing-dialog-content'
      data-dialog-id={uniqueId}
      style={style}
      role='dialog'
      aria-modal='true'
      aria-labelledby={`motion-ui-morphing-dialog-title-${uniqueId}`}
      aria-describedby={`motion-ui-morphing-dialog-description-${uniqueId}`}
    >
      {children}
    </motion.div>
  );
}

export type MorphingDialogContainerProps = {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
};

function MorphingDialogContainer({ children }: MorphingDialogContainerProps) {
  const { isOpen, uniqueId } = useMorphingDialog();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    // Schedule state update for next tick to avoid synchronous update warning
    const timer = setTimeout(() => setMounted(true), 0);
    return () => {
      clearTimeout(timer);
      setMounted(false);
    };
  }, []);

  if (!mounted) return null;

  return createPortal(
    <AnimatePresence initial={false} mode='sync'>
      {isOpen && (
        <>
          <motion.div
            key={`backdrop-${uniqueId}`}
            className='fixed inset-0 z-50 bg-black/40 backdrop-blur-sm'
            data-slot='morphing-dialog-layer'
            data-dialog-id={uniqueId}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0, transition: { duration: 0.2 } }}
          />
          <div
            className='fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-2 sm:items-center sm:p-4'
            data-slot='morphing-dialog-layer'
            data-dialog-id={uniqueId}
          >
            {children}
          </div>
        </>
      )}
    </AnimatePresence>,
    document.body
  );
}

export type MorphingDialogTitleProps = {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
};

function MorphingDialogTitle({
  children,
  className,
  style,
}: MorphingDialogTitleProps) {
  const { uniqueId } = useMorphingDialog();

  return (
    <motion.div
      layoutId={`dialog-title-container-${uniqueId}`}
      className={className}
      style={style}
      layout
    >
      {children}
    </motion.div>
  );
}

export type MorphingDialogSubtitleProps = {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
};

function MorphingDialogSubtitle({
  children,
  className,
  style,
}: MorphingDialogSubtitleProps) {
  const { uniqueId } = useMorphingDialog();

  return (
    <motion.div
      layoutId={`dialog-subtitle-container-${uniqueId}`}
      className={className}
      style={style}
    >
      {children}
    </motion.div>
  );
}

export type MorphingDialogDescriptionProps = {
  children: React.ReactNode;
  className?: string;
  disableLayoutAnimation?: boolean;
  variants?: {
    initial: Variant;
    animate: Variant;
    exit: Variant;
  };
};

function MorphingDialogDescription({
  children,
  className,
  variants,
  disableLayoutAnimation,
}: MorphingDialogDescriptionProps) {
  const { uniqueId } = useMorphingDialog();

  return (
    <motion.div
      key={`dialog-description-${uniqueId}`}
      layoutId={
        disableLayoutAnimation
          ? undefined
          : `dialog-description-content-${uniqueId}`
      }
      variants={variants}
      className={className}
      initial='initial'
      animate='animate'
      exit='exit'
      id={`dialog-description-${uniqueId}`}
    >
      {children}
    </motion.div>
  );
}

export type MorphingDialogImageProps = {
  src: string;
  alt: string;
  className?: string;
  style?: React.CSSProperties;
};

function MorphingDialogImage({
  src,
  alt,
  className,
  style,
}: MorphingDialogImageProps) {
  const { uniqueId } = useMorphingDialog();

  return (
    <motion.img
      src={src}
      alt={alt}
      className={cn(className)}
      layoutId={`dialog-img-${uniqueId}`}
      style={style}
    />
  );
}

export type MorphingDialogCloseProps = {
  children?: React.ReactNode;
  className?: string;
  variants?: {
    initial: Variant;
    animate: Variant;
    exit: Variant;
  };
};

function MorphingDialogClose({
  children,
  className,
  variants,
}: MorphingDialogCloseProps) {
  const t = useTranslations('common.dialog');
  const { setIsOpen, uniqueId } = useMorphingDialog();

  const handleClose = useCallback(() => {
    setIsOpen(false);
  }, [setIsOpen]);

  return (
    <motion.button
      onClick={handleClose}
      type='button'
      aria-label={t('close')}
      key={`dialog-close-${uniqueId}`}
      className={cn(
        'absolute top-2 right-2 sm:top-4 sm:right-4 flex items-center justify-center rounded-md border border-border bg-card p-2 sm:p-1.5 min-w-[44px] min-h-[44px] sm:min-w-0 sm:min-h-0 opacity-80 transition-all duration-150 hover:opacity-100 hover:bg-muted',
        className
      )}
      initial='initial'
      animate='animate'
      exit='exit'
      variants={variants}
    >
      {children || <XIcon size={24} />}
    </motion.button>
  );
}

export {
  MorphingDialog,
  MorphingDialogTrigger,
  MorphingDialogContainer,
  MorphingDialogContent,
  MorphingDialogClose,
  MorphingDialogTitle,
  MorphingDialogSubtitle,
  MorphingDialogDescription,
  MorphingDialogImage,
  useMorphingDialog,
};
