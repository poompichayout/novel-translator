const { chromium } = require('playwright');

(async () => {
    const url = process.argv[2];
    const timeoutMsg = process.argv[3] ? parseInt(process.argv[3]) : 30000;

    if (!url) {
        console.error('URL argument is required');
        process.exit(1);
    }

    let browser;
    try {
        browser = await chromium.launch({
            headless: true,
            args: ['--no-sandbox', '--disable-setuid-sandbox']
        });
        
        const context = await browser.newContext({
            userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            viewport: { width: 1280, height: 720 }
        });

        const page = await context.newPage();
        
        // Block media and images to speed up rendering
        await page.route('**/*.{png,jpg,jpeg,gif,svg,css,woff,woff2}', route => route.abort());
        
        await page.goto(url, { waitUntil: 'domcontentloaded', timeout: timeoutMsg });
        
        // Wait a bit extra for dynamic frameworks to attach
        await page.waitForTimeout(1000);

        const content = await page.content();
        console.log(content);
        
    } catch (error) {
        console.error(`Error rendering page: ${error.message}`);
        process.exit(1);
    } finally {
        if (browser) {
            await browser.close();
        }
    }
})();
