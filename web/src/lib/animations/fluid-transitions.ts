import type { Variants } from 'motion/react';

export const EASING = {
    easeOutCubic: [0.25, 0.46, 0.45, 0.94] as const,
    easeOutExpo: [0.16, 1, 0.3, 1] as const,
    easeOutQuart: [0.25, 1, 0.5, 1] as const,
    easeOutBack: [0.34, 1.56, 0.64, 1] as const,
    easeInOutCubic: [0.65, 0, 0.35, 1] as const,
    easeInOutQuart: [0.76, 0, 0.24, 1] as const,
    easeInCubic: [0.32, 0, 0.67, 0] as const,
    natureRipple: [0.25, 0.46, 0.45, 0.94] as const,
    waterhouseDrift: [0.25, 0.46, 0.45, 0.94] as const,
    waterhouseFloat: [0.16, 1, 0.3, 1] as const,
    waterhouseSurface: [0.16, 1, 0.3, 1] as const,
} as const;

export const SPRING = {
    smooth: {
        type: "spring" as const,
        stiffness: 80,
        damping: 20,
        mass: 1.2,
    },
    gentle: {
        type: "spring" as const,
        stiffness: 70,
        damping: 18,
        mass: 1.5,
    },
    bouncy: {
        type: "spring" as const,
        stiffness: 100,
        damping: 15,
        mass: 1,
    },
    waterhouseHover: {
        type: "spring" as const,
        stiffness: 120,
        damping: 18,
        mass: 1,
    },
    waterhouseMagnetic: {
        type: "spring" as const,
        stiffness: 60,
        damping: 22,
        mass: 1.4,
    },
    snappy: {
        type: "spring" as const,
        stiffness: 180,
        damping: 16,
        mass: 0.8,
    },
} as const;

export const ENTRANCE_VARIANTS = {
    navbar: {
        initial: {
            opacity: 0,
            scale: 0.95,
        },
        animate: {
            opacity: 1,
            scale: 1,
            transition: SPRING.gentle,
        },
    } as Variants,

    content: {
        initial: {
            opacity: 0,
        },
        animate: {
            opacity: 1,
            transition: {
                duration: 0.2,
                ease: EASING.easeOutCubic,
                delay: 0.04,
            },
        },
    } as Variants,
} as const;

export const EXIT_VARIANTS = {
    content: {
        exit: {
            opacity: 0,
            transition: {
                duration: 0.15,
                ease: EASING.easeInCubic,
            },
        },
    } as Variants,
} as const;

export const SLIDE_VARIANTS = {
    fadeInUp: {
        initial: {
            opacity: 0,
            y: 20,
        },
        animate: {
            opacity: 1,
            y: 0,
            transition: {
                duration: 0.25,
                ease: EASING.easeOutQuart,
            },
        },
        exit: {
            opacity: 0,
            y: -10,
            transition: {
                duration: 0.15,
                ease: EASING.easeInCubic,
            },
        },
    } as Variants,

    scaleIn: {
        initial: {
            opacity: 0,
            scale: 0.92,
        },
        animate: {
            opacity: 1,
            scale: 1,
            transition: SPRING.smooth,
        },
        exit: {
            opacity: 0,
            scale: 0.95,
            transition: {
                duration: 0.12,
                ease: EASING.easeInCubic,
            },
        },
    } as Variants,
} as const;

export const STAGGER_CONTAINER = {
    animate: {
        transition: {
            staggerChildren: 0.05,
            delayChildren: 0.08,
        },
    },
} as Variants;
