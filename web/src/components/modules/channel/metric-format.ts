type MetricDisplayInput = {
    raw: number;
    formatted: {
        value: string;
        unit: string;
    };
};

export function getChannelMetricDisplayParts(metric: MetricDisplayInput) {
    const unit = metric.formatted.unit.trim();

    return {
        value: metric.formatted.value,
        unit,
        text: unit ? `${metric.formatted.value} ${unit}` : metric.formatted.value,
    };
}
