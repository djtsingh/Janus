// static/sensor.js
console.log('Starting sensor.js');

async function collectFingerprint() {
    console.log('collectFingerprint: Starting fingerprint collection');

    // Collect fingerprint data
    const canvas = document.createElement('canvas');
    const ctx = canvas.getContext('2d');
    canvas.width = 200;
    canvas.height = 50;
    ctx.textBaseline = 'top';
    ctx.font = '14px Arial';
    ctx.fillText('fingerprint', 2, 2);
    const canvasHash = canvas.toDataURL();

    const plugins = Array.from(navigator.plugins).map(p => p.name).join(',');
    const screenRes = `${screen.width}x${screen.height}`;
    const colorDepth = screen.colorDepth;
    const fonts = (function () {
        const testFonts = ['Arial', 'Times New Roman', 'Helvetica'];
        return testFonts.filter(font => document.fonts.check(`12px "${font}"`)).join(',');
    })();
    const webgl = (function () {
        const gl = document.createElement('canvas').getContext('webgl');
        if (!gl) return 'no-webgl';
        return gl.getParameter(gl.RENDERER);
    })();
    const isMobile = /Mobi|Android/i.test(navigator.userAgent);
    const fingerprint = {
        plugins: plugins,
        hardwareCon: navigator.hardwareConcurrency || 0,
        webdriver: !!navigator.webdriver,
        chromeExists: !!window.chrome,
        canvas_Hash: canvasHash,
        screenRes: screenRes,
        colorDepth: colorDepth,
        fonts: fonts,
        webglRenderer: webgl,
        ja3: 'unknown-ja3',
        screen: { width: screen.width, height: screen.height },
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
        jsEnabled: true,
        isMobile: isMobile
    };

    console.log('collectFingerprint: Canvas hash generated: ' + fingerprint.canvasHash);
    console.log('collectFingerprint: Fonts detected: ' + fingerprint.fonts);
    console.log('collectFingerprint: WebGL renderer: ' + fingerprint.webglRenderer);

    try {
        // Send fingerprint
        console.log('collectFingerprint: Sending fingerprint to /janus/fingerprint');
        let response = await fetch('/janus/fingerprint', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(fingerprint)
        });
        if (!response.ok) throw new Error('Fingerprint submission failed: ' + response.status);

        console.log('collectFingerprint: Fingerprint submitted successfully');

        // Fetch challenge
        console.log('collectFingerprint: Fetching challenge from /janus/challenge');
        response = await fetch('/janus/challenge');
        if (!response.ok) throw new Error('Challenge fetch failed: ' + response.status);
        const challenge = await response.json();
        console.log('collectFingerprint: Received challenge: ' + JSON.stringify(challenge));

        // Compute hashcash proof
        const { nonce, iterations, seed, clientIP, zeroBits } = challenge;
        const timestamp = new Date().toISOString();

        let proof;
        const maxIterations = Math.min(iterations, isMobile ? 1000 : 5000);
        
        for (let i = 0; i < maxIterations; i++) {
            proof = `${nonce}|${i}|${timestamp}|${clientIP}|${seed}`;
            if (!isMobile) {
                proof += `|${canvasHash}`;
            }
            const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(proof));
            const hashArray = new Uint8Array(hash);
            if (hasLeadingZeroBits(hashArray, zeroBits)) {
                console.log('collectFingerprint: Computed proof: ' + proof);
                break;
            }
            if (i === maxIterations - 1) {
                throw new Error('Failed to compute valid proof within iteration limit');
            }
        }

        // Verify proof
        console.log('collectFingerprint: Sending proof to /janus/verify: ' + proof);
        response = await fetch('/janus/verify', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ nonce, proof })
        });
        if (!response.ok) throw new Error('Verification failed: ' + response.status);
        const verifyResult = await response.json();
        if (verifyResult.status !== 'success') throw new Error('Verification status not success');

        console.log('collectFingerprint: Verification successful');
        window.location.href = '/';
    } catch (error) {
        console.error('collectFingerprint: Error in fingerprint/challenge flow: ' + error.message);
        document.getElementById('status').textContent = 'Verification failed, please refresh to try again.';
    }
}

function hasLeadingZeroBits(hash, zeroBits) {
    const fullBytes = Math.floor(zeroBits / 8);
    const extraBits = zeroBits % 8;
    for (let i = 0; i < fullBytes; i++) {
        if (hash[i] !== 0) return false;
    }
    if (extraBits > 0) {
        const mask = 0xFF << (8 - extraBits);
        return (hash[fullBytes] & mask) === 0;
    }
    return true;
}

document.addEventListener('DOMContentLoaded', () => {
    console.log('DOMContentLoaded: Triggering collectFingerprint');
    if (!window.fingerprintProcessed) {
        window.fingerprintProcessed = true;
        collectFingerprint();
    }
});