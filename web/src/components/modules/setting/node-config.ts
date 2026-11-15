export function buildNodeCloudURL(serverURL: string): string {
    const url = new URL(serverURL);
    const isLoopback = url.hostname === 'localhost' || url.hostname === '127.0.0.1' || url.hostname === '[::1]';
    url.protocol = url.protocol === 'http:' && isLoopback ? 'ws:' : 'wss:';
    if (url.protocol === 'ws:' && url.hostname === 'localhost') url.hostname = '127.0.0.1';
    url.pathname = `${url.pathname.replace(/\/$/, '')}/node/v1/connect`;
    url.search = '';
    url.hash = '';
    return url.toString();
}

export function buildNodeConfig(token: string, serverURL: string): string {
    const cloudURL = buildNodeCloudURL(serverURL);
    const allowInsecureLoopback = cloudURL.startsWith('ws://127.0.0.1:') || cloudURL.startsWith('ws://127.0.0.1/');
    return JSON.stringify({
        plugin_options: {
            version: 1,
            connect: false,
            cloud_url: cloudURL,
            models: ['<MODEL_ID>'],
            max_concurrency: 4,
            request_timeout_seconds: 300,
        },
        private_config: {
            node_token: token,
            magpie_url: 'http://127.0.0.1:3425',
            magpie_key: '<LOCAL_MAGPIE_GATEWAY_KEY>',
            ...(allowInsecureLoopback ? { allow_insecure_loopback: true } : {}),
        },
    }, null, 2);
}
