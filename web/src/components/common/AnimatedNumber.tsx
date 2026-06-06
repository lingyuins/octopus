import { useEffect, useState, useRef } from 'react';
import { animate } from 'motion/react';
import { useIsMobile } from '@/hooks/use-mobile';

interface AnimatedNumberProps {
    value: string | number | undefined;
    duration?: number;
}

export function AnimatedNumber({ value, duration = 800 }: AnimatedNumberProps) {
    const [displayValue, setDisplayValue] = useState(0);
    const prevValueRef = useRef(0);
    const isMobile = useIsMobile();

    useEffect(() => {
        if (value === undefined || value === null || value === '-') {
            prevValueRef.current = 0;
            return;
        }

        const numericValue = typeof value === 'string'
            ? parseFloat(value.replace(/,/g, ''))
            : value;

        if (isNaN(numericValue)) {
            return;
        }

        const controls = animate(prevValueRef.current, numericValue, {
            duration: duration / 1000, // motion/react uses seconds
            ease: 'easeOut',
            onUpdate: (latest) => {
                setDisplayValue(latest);
                prevValueRef.current = latest;
            }
        });

        return () => controls.stop();
    }, [value, duration]);

    if (value === undefined || value === null) {
        return <span>-</span>;
    }

    const shouldShowDecimals = typeof value === 'string' && value.includes('.');
    const decimalPlaces = shouldShowDecimals ? 2 : 0;

    const formattedValue = displayValue.toLocaleString('en-US', {
        notation: isMobile && displayValue >= 1_000_000 ? 'compact' : 'standard',
        maximumFractionDigits: decimalPlaces,
        ...(shouldShowDecimals ? { minimumFractionDigits: decimalPlaces } : {}),
    });

    return <span>{formattedValue}</span>;
}