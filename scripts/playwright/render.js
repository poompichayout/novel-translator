const { chromium } = require('playwright-extra');
const stealth = require('puppeteer-extra-plugin-stealth')();

chromium.use(stealth);
(async () => {
    const url = process.argv[2];
    const timeoutMsg = process.argv[3] ? parseInt(process.argv[3]) : 30000;

    if (!url) {
        console.error('URL argument is required');
        process.exit(1);
    }

    let context;
    try {
        const userDataDir = "/tmp/playwright-session";

        // Launch a persistent context. This is much stronger against Cloudflare since it 
        // saves Cache, LocalStorage, IndexedDB, and Cookies between runs.
        context = await chromium.launchPersistentContext(userDataDir, {
            headless: false, // Must be true for debugging and true human-likeness
            args: [
                '--no-sandbox',
                '--disable-setuid-sandbox',
                '--disable-blink-features=AutomationControlled',
                '--start-maximized'
            ],
            userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            viewport: {
                width: 1280 + Math.floor(Math.random() * 100),
                height: 720 + Math.floor(Math.random() * 100)
            },
            locale: "en-US",
            timezoneId: "America/New_York",
        });

        // persistentContext already comes with one open page by default
        const page = context.pages().length > 0 ? context.pages()[0] : await context.newPage();

        await page.goto(url, { waitUntil: 'domcontentloaded', timeout: timeoutMsg });

        // Cloudflare Challenge bypass: wait until the title is no longer "Just a moment..."
        let title = await page.title();
        if (title.includes('Just a moment')) {
            console.log("Cloudflare Challenge detected. Waiting for auto-verify...");

            // Look for any visible challenge iframes and ensure they finish rendering
            const frames = page.frames();
            for (const frame of frames) {
                if (frame.url().includes('cloudflare')) {
                    // Let the frame do its work (sometimes requires clicking but stealth mode mostly auto-solves it)
                    await page.waitForTimeout(2000);
                }
            }

            // Wait for the verification spinner/div to disappear completely
            await page.waitForFunction(() => {
                const isStillWaiting = document.title.includes('Just a moment');
                const isSpinnerVisible = document.querySelector('.loading-verifying') !== null;
                return !isStillWaiting && !isSpinnerVisible;
            }, { timeout: 30000 }).catch(() => { });

            // Wait for any challenge iframes to be removed and the page to actually redirect
            await page.waitForTimeout(5000);
        } else {
            // Wait a bit extra for dynamic frameworks to attach
            await page.waitForTimeout(2000);
        }

        const content = await page.content();
        console.log(content);



    } catch (error) {
        console.error(`Error rendering page: ${error.message}`);
        process.exit(1);
    } finally {
        if (context) {
            await context.close();
        }
        process.exit(0);
    }
})();
