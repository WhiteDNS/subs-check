const MIRROR_URL = 'https://christianai.pages.dev';

const createResponse = (data, status = 200, contentType = 'application/json') => {
    const body = contentType === 'application/json' ? JSON.stringify(data) : data;
    return new Response(body, {
        status,
        headers: {
            'Content-Type': `${contentType}; charset=utf-8`,
            'Access-Control-Allow-Origin': '*',
            'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
            'Access-Control-Allow-Headers': 'Content-Type, Authorization'
        }
    });
};

const handleError = (message, status = 500) => {
    return createResponse({ code: status, message }, status);
};

const routeHandlers = {

    async github(request, url) {
        try {
            const githubPath = url.pathname.replace('/github/', '');
            const githubUrl = `https://api.github.com/${githubPath}`;
            const headers = new Headers(request.headers);
            headers.set('User-Agent', 'Cloudflare-Worker');

            const githubResponse = await fetch(githubUrl, {
                method: request.method,
                headers,
                body: request.method !== 'GET' ? await request.text() : undefined
            });

            return new Response(await githubResponse.text(), {
                status: githubResponse.status,
                headers: {
                    'Access-Control-Allow-Origin': '*',
                    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
                    'Access-Control-Allow-Headers': 'Content-Type, Authorization',
                    'Content-Type': 'application/json'
                }
            });
        } catch (error) {
            return handleError('GitHub API request failed: ' + error.message);
        }
    },

    async gist(request, url, env) {
        if (!await validateToken(url, env)) {
            return handleError('Unauthorized access', 401);
        }

        try {
            const key = url.searchParams.get('key');
            const timestamp = Date.now();
            const gistUrl = `https://gist.githubusercontent.com/${env.GITHUB_USER}/${env.GITHUB_ID}/raw/${key}?timestamp=${timestamp}`;
            const gistContent = await fetch(gistUrl).then(res => res.text());
            return createResponse(gistContent, 200, 'text/plain');
        } catch (error) {
            return handleError('Failed to get Gist content: ' + error.message);
        }
    },

    async storage(request, url, env) {
        if (!await validateToken(url, env)) {
            return handleError('Unauthorized access', 401);
        }

        if (request.method === 'GET') {
            const filename = url.searchParams.get('filename');
            if (!filename) {
                return handleError('Please provide a filename', 400);
            }

            try {
                const object = await env.SUB_BUCKET.get(filename);
                if (object === null) {
                    return handleError('No value found for this key', 404);
                }
                return createResponse(await object.text(), 200, 'text/plain');
            } catch (error) {
                return handleError('Failed to read data: ' + error.message);
            }
        } else if (request.method === 'POST') {
            try {
                const { filename, value } = await request.json();
                if (!filename || !value) {
                    return handleError('Please provide a filename and value', 400);
                }

                await env.SUB_BUCKET.put(filename, value);
                return createResponse({ code: 200, message: 'Data written successfully' });
            } catch (error) {
                return handleError('Failed to write data: ' + error.message);
            }
        }

        return handleError('Unsupported request method', 405);
    },
    async speedtest(request, url, env) {
        try {
            const bytes = url.searchParams.get('bytes');
            if (!bytes) {
                return handleError('Please provide the test size in bytes', 400);
            }

            const speedTestUrl = `https://speed.cloudflare.com/__down?bytes=${bytes}`;
            const response = await fetch(speedTestUrl, {
                method: request.method,
                headers: request.headers
            });

            return new Response(response.body, {
                status: response.status,
                headers: {
                    'Access-Control-Allow-Origin': '*',
                    'Access-Control-Allow-Methods': 'GET, OPTIONS',
                    'Access-Control-Allow-Headers': 'Content-Type',
                    'Content-Type': 'application/octet-stream'
                }
            });
        } catch (error) {
            return handleError('Speed test failed: ' + error.message);
        }
    },
    async raw(request, url) {
        try {
            // Extract the part after /raw from pathname.
            const inputPath = url.pathname.replace('/raw', '');
            if (!inputPath || inputPath == '/') {
                return handleError('Please provide a GitHub-related path', 400);
            }
    
            let targetUrl;
            // Check whether the path includes a domain.
            if (inputPath.includes('raw.githubusercontent.com')) {
                // Extract the path after raw.githubusercontent.com.
                const rawIndex = inputPath.indexOf('raw.githubusercontent.com');
                const githubPath = inputPath.substring(rawIndex + 'raw.githubusercontent.com'.length);
                targetUrl = `https://raw.githubusercontent.com${githubPath}`;
            } else if (inputPath.includes('github.com')) {
                // Extract the path after github.com (release or archive).
                const githubIndex = inputPath.indexOf('github.com');
                const githubPath = inputPath.substring(githubIndex + 'github.com'.length);
                if (githubPath.includes('/releases/download/') || githubPath.includes('/archive/')) {
                    targetUrl = `https://github.com${githubPath}`;
                } else {
                    return handleError('Only raw file, release, or archive paths are supported', 400);
                }
            } else {
                // No domain: assume this is a raw file path or a release/archive path.
                const path = inputPath.startsWith('/') ? inputPath : `/${inputPath}`;
                if (path.includes('/releases/download/') || path.includes('/archive/')) {
                    targetUrl = `https://github.com${path}`;
                } else {
                    targetUrl = `https://raw.githubusercontent.com${path}`;
                }
            }
    
            // Set request headers.
            const headers = new Headers(request.headers);
            headers.set('User-Agent', 'Cloudflare-Worker');
    
            // Download through the Cloudflare proxy.
            const response = await fetch(targetUrl, {
                method: 'GET',
                headers
            });
    
            if (!response.ok) {
                return handleError('GitHub download failed', response.status);
            }
    
            const contentType = response.headers.get('Content-Type') || 'application/octet-stream';
            return new Response(response.body, {
                status: response.status,
                headers: {
                    'Access-Control-Allow-Origin': '*',
                    'Access-Control-Allow-Methods': 'GET, OPTIONS',
                    'Access-Control-Allow-Headers': 'Content-Type',
                    'Content-Type': contentType
                }
            });
        } catch (error) {
            return handleError('GitHub proxy failed: ' + error.message);
        }
    }
};

async function validateToken(url, env) {
    const token = url.searchParams.get('token');
    return token === env.AUTH_TOKEN;
}

async function handleMirrorRequest(request, url) {
    try {
        const clockieUrl = new URL(url.pathname + url.search, MIRROR_URL);
        const response = await fetch(clockieUrl.toString(), {
            method: request.method,
            headers: request.headers,
            body: request.method !== 'GET' ? await request.clone().text() : undefined
        });

        const responseHeaders = new Headers(response.headers);
        responseHeaders.set('Access-Control-Allow-Origin', '*');

        return new Response(await response.text(), {
            status: response.status,
            headers: responseHeaders
        });
    } catch (error) {
        return handleError('Mirror request failed: ' + error.message);
    }
}

export default {
    async fetch(request, env) {
        try {
            const url = new URL(request.url);
            const pathname = url.pathname;

            const routes = {
                '/github/': () => routeHandlers.github(request, url),
                '/gist': () => routeHandlers.gist(request, url, env),
                '/storage': () => routeHandlers.storage(request, url, env),
                '/speedtest': () => routeHandlers.speedtest(request, url, env),
                '/raw': () => routeHandlers.raw(request, url)
            };

            for (const [route, handler] of Object.entries(routes)) {
                if (pathname === route || pathname.startsWith(route)) {
                    return await handler();
                }
            }

            return await handleMirrorRequest(request, url);
        } catch (error) {
            return handleError('Server error: ' + error.message);
        }
    }
};
