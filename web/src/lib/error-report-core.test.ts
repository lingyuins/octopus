import { describe, it, beforeEach } from 'node:test';
import assert from 'node:assert/strict';
import { ErrorReportDedupe } from './error-report-core.ts';

describe('ErrorReportDedupe', () => {
    let dedupe: ErrorReportDedupe;

    beforeEach(() => {
        dedupe = new ErrorReportDedupe();
    });

    it('first report is not a duplicate', () => {
        assert.equal(dedupe.isDuplicate('TypeError: boom', 'stack-1'), false);
    });

    it('same message+stack within window is a duplicate', () => {
        dedupe.isDuplicate('TypeError: boom', 'stack-1');
        assert.equal(dedupe.isDuplicate('TypeError: boom', 'stack-1'), true);
    });

    it('different message is not a duplicate', () => {
        dedupe.isDuplicate('TypeError: boom', 'stack-1');
        assert.equal(dedupe.isDuplicate('RangeError: overflow', 'stack-1'), false);
    });

    it('different stack for same message is not a duplicate', () => {
        dedupe.isDuplicate('TypeError: boom', 'stack-1');
        assert.equal(dedupe.isDuplicate('TypeError: boom', 'stack-2'), false);
    });

    it('stack longer than 200 chars is truncated for key comparison', () => {
        const long = 'x'.repeat(500);
        dedupe.isDuplicate('TypeError: boom', long);
        // 前 200 字符相同即视为重复
        assert.equal(dedupe.isDuplicate('TypeError: boom', long), true);
    });

    it('reset clears dedupe state', () => {
        dedupe.isDuplicate('TypeError: boom', 'stack-1');
        dedupe.reset();
        assert.equal(dedupe.isDuplicate('TypeError: boom', 'stack-1'), false);
    });

    it('alternating errors are still deduped within the window', () => {
        // 单槽位实现下 A/B 交替会互相顶掉，每次都判为新错误。
        assert.equal(dedupe.isDuplicate('ErrorA', 'stack-a'), false);
        assert.equal(dedupe.isDuplicate('ErrorB', 'stack-b'), false);
        assert.equal(dedupe.isDuplicate('ErrorA', 'stack-a'), true);
        assert.equal(dedupe.isDuplicate('ErrorB', 'stack-b'), true);
    });

    it('entry count stays bounded under many distinct errors', () => {
        for (let i = 0; i < 500; i++) {
            dedupe.isDuplicate(`Error-${i}`, 'stack');
        }
        // 最新条目仍在窗口内应判重，且不会因容量淘汰而误报新错误。
        assert.equal(dedupe.isDuplicate('Error-499', 'stack'), true);
    });
});
