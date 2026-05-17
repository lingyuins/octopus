export type TelemetryMetricTone = 'success' | 'warning' | 'danger';

const telemetryPercentPrecision = 1;
const telemetryErrorWarningThreshold = 5;
const telemetryErrorDangerThreshold = 10;

export function formatTelemetryPercent(value: number): string {
    return `${value.toFixed(telemetryPercentPrecision)}%`;
}

export function getTelemetryErrorRateTone(errorRatePercent: number): TelemetryMetricTone {
    if (errorRatePercent > telemetryErrorDangerThreshold) {
        return 'danger';
    }
    if (errorRatePercent > telemetryErrorWarningThreshold) {
        return 'warning';
    }
    return 'success';
}
