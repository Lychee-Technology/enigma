/**
* Mock implementation of the fetch API that only handles data URLs
* @param {string} url - The URL to fetch (must be a data URL)
* @param {Object} options - Fetch options (optional)
* @returns {Promise<Response>} - A promise that resolves to a Response object
*/
export function mockFetchBase64DataUrl(url, options = {}) {
    const BIN_MIME = "application/octet-stream";
    return new Promise((resolve, reject) => {
        // Check if the URL is a data URL
        if (!url.startsWith('data:')) {
            reject(new Error('This mock fetch only supports data URLs (starting with "data:")'));
            return;
        }

        try {
            // Parse the data URL
            const [header, encodedData] = url.split(',');
            const isBase64 = header.includes(';base64');
            if (!isBase64) {
                reject(new Error('Only base64 data URLs are supported'));
                return;
            }

            const mimeType = BIN_MIME;

            // Decode the data
            const decodedData = Buffer.from(encodedData, 'base64');

            // Convert string to ArrayBuffer/Blob as needed
            const blob = new Blob([decodedData], { type: mimeType });

            // Create a mock Response object
            const response = {
                ok: true,
                status: 200,
                statusText: 'OK',
                headers: new Headers({
                    'Content-Type': mimeType,
                    'Content-Length': blob.size
                }),
                url: url,
                redirected: false,
                type: 'basic',
                body: null,
                bodyUsed: false,

                // Response methods
                text: () => Promise.resolve(decodedData),
                json: () => {
                    try {
                        return Promise.resolve(JSON.parse(decodedData));
                    } catch (e) {
                        return Promise.reject(new Error('Failed to parse JSON'));
                    }
                },
                blob: () => Promise.resolve(blob),
                arrayBuffer: () => Promise.resolve(decodedData),
                clone: () =>
                    // Create a deep copy of the response
                    Object.assign(Object.create(Object.getPrototypeOf(this)), this)
            };

            // Resolve with the mock Response
            setTimeout(() => {
                resolve(response);
            }, 0); // Simulate asynchronous behavior
        } catch (error) {
            reject(new Error(`Failed to process data URL: ${error.message}`));
        }
    });
}
