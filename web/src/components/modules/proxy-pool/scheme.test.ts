import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    detectProxyScheme,
    maskProxyURL,
    buildSchemeTemplate,
    schemeLabel,
    PROXY_SCHEMES,
} from './scheme.ts';

test('detectProxyScheme recognizes each supported scheme', () => {
    const cases: Array<[string, string | null]> = [
        ['http://127.0.0.1:7890', 'http'],
        ['https://proxy.example.com:8443', 'https'],
        ['socks5://127.0.0.1:1080', 'socks5'],
        ['ss://YWVzLTI1Ni1nY206cGFzcw@example.com:8388', 'ss'],
        ['vmess://eyJhZGQiOiJleGFtcGxlLmNvbSJ9', 'vmess'],
        ['vless://uuid@example.com:443?security=tls', 'vless'],
        ['trojan://pass@example.com:443', 'trojan'],
        // legacy alias → socks5
        ['socks://127.0.0.1:1080', 'socks5'],
        // case-insensitive scheme
        ['SOCKS5://127.0.0.1:1080', 'socks5'],
        ['HTTP://127.0.0.1:7890', 'http'],
        // unsupported / malformed
        ['ftp://host:21', null],
        ['quic://host:443', null],
        ['', null],
        ['  ', null],
        ['example.com:8080', null],
    ];
    for (const [input, expected] of cases) {
        assert.equal(detectProxyScheme(input), expected, `detectProxyScheme(${JSON.stringify(input)})`);
    }
});

test('detectProxyScheme requires the :// separator', () => {
    assert.equal(detectProxyScheme('http:127.0.0.1'), null);
    assert.equal(detectProxyScheme('socks5 127.0.0.1'), null);
});

test('PROXY_SCHEMES covers the full supported set', () => {
    assert.deepEqual(PROXY_SCHEMES, ['http', 'https', 'socks5', 'ss', 'vmess', 'vless', 'trojan']);
});

test('buildSchemeTemplate returns a usable example per scheme', () => {
    for (const scheme of PROXY_SCHEMES) {
        const template = buildSchemeTemplate(scheme);
        assert.ok(template.startsWith(`${scheme}://`), `${scheme} template starts with scheme`);
        assert.equal(detectProxyScheme(template), scheme, `${scheme} template re-detects`);
    }
});

test('schemeLabel returns a display name', () => {
    assert.equal(schemeLabel('http'), 'HTTP');
    assert.equal(schemeLabel('socks5'), 'SOCKS5');
    assert.equal(schemeLabel('ss'), 'SS');
    assert.equal(schemeLabel('vmess'), 'VMess');
});

test('maskProxyURL masks credentials in the username slot (trojan/ss/vless)', () => {
    assert.ok(!maskProxyURL('trojan://secret-pass@example.com:443').includes('secret-pass'));
    assert.ok(!maskProxyURL('ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ@example.com:8388').includes('YWVzLTI1Ni1nY206cGFzc3dvcmQ'));
    assert.ok(!maskProxyURL('vless://11111111-2222-3333-4444-555555555555@example.com:443').includes('11111111'));
    // host 仍可见，便于辨认条目。
    assert.ok(maskProxyURL('trojan://secret-pass@example.com:443').includes('example.com'));
});

test('maskProxyURL masks the whole vmess payload and userinfo-less ss links', () => {
    assert.equal(maskProxyURL('vmess://eyJhZGQiOiJleGFtcGxlLmNvbSIsImlkIjoidXVpZCJ9'), 'vmess://***');
    assert.equal(maskProxyURL('ss://YWVzOnBhc3NAZXhhbXBsZS5jb206ODM4OA=='), 'ss://***');
});

test('maskProxyURL keeps masking http/socks5 passwords, preserves username', () => {
    const masked = maskProxyURL('http://user:secret@127.0.0.1:7890');
    assert.ok(!masked.includes('secret'));
    assert.ok(masked.includes('user'));
    assert.equal(maskProxyURL('socks5://127.0.0.1:1080'), 'socks5://127.0.0.1:1080');
    assert.equal(maskProxyURL('not a url'), 'not a url');
});
