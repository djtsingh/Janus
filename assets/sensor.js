console.log('Starting sensor.js');

async function collectFingerprint() {
    console.log('collectFingerprint: Starting fingerprint collection');

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
        console.log('collectFingerprint: Sending fingerprint to /janus/fingerprint');
        let response = await fetch('/janus/fingerprint', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(fingerprint)
        });
        if (!response.ok) throw new Error('Fingerprint submission failed: ' + response.status);

        console.log('collectFingerprint: Fingerprint submitted successfully');

        console.log('collectFingerprint: Fetching challenge from /janus/challenge');
        response = await fetch('/janus/challenge');
        if (!response.ok) throw new Error('Challenge fetch failed: ' + response.status);
        const challenge = await response.json();
        console.log('collectFingerprint: Received challenge: ' + JSON.stringify(challenge));

        // Extract challenge params early so they're available for all challenge types
        const { nonce, iterations, seed, clientIP, difficulty } = challenge;

        // Define verifyProof early so it's available for all challenge type handlers
        async function verifyProof(proofVal) {
            console.log('collectFingerprint: Sending proof to /janus/verify: ' + proofVal);
            try {
                let verifyResponse = await fetch('/janus/verify', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ nonce, proof: proofVal })
                });
                if (!verifyResponse.ok) {
                    if (verifyResponse.status === 400) {
                        // Challenge expired - offer refresh
                        document.getElementById('status').innerHTML = 
                            'Challenge expired. <a href="javascript:location.reload()">Click here to retry</a>';
                        return;
                    }
                    throw new Error('Verification failed: ' + verifyResponse.status);
                }
                const verifyResult = await verifyResponse.json();
                if (verifyResult.status !== 'success') throw new Error('Verification status not success');
                console.log('collectFingerprint: Verification successful');
                window.location.href = '/';
            } catch (err) {
                console.error('collectFingerprint: Verification error:', err);
                document.getElementById('status').innerHTML = 
                    'Verification failed. <a href="javascript:location.reload()">Click here to retry</a>';
            }
        }

        // Challenge timeout tracking (server expires in 5 minutes)
        const challengeExpiry = Date.now() + (4.5 * 60 * 1000); // 4.5 min client-side (buffer)
        function updateCountdown(element) {
            const remaining = Math.max(0, Math.floor((challengeExpiry - Date.now()) / 1000));
            if (remaining <= 0) {
                element.innerHTML = '<span style="color:red">Challenge expired. <a href="javascript:location.reload()">Refresh to retry</a></span>';
                return false;
            }
            const mins = Math.floor(remaining / 60);
            const secs = remaining % 60;
            element.textContent = `Time remaining: ${mins}:${secs.toString().padStart(2, '0')}`;
            return true;
        }

        const challengeUI = document.getElementById('challenge-ui');
        if (challenge.type === 'image') {
            challengeUI.innerHTML = `
                <b>Image Puzzle:</b> Click the cat image to continue.<br>
                <img id="cat-img" src="https://cataas.com/cat?width=120" style="cursor:pointer;max-width:120px;margin:10px 0;">
                <div id="countdown" style="font-size:12px;color:#666;"></div>
            `;
            document.getElementById('cat-img').onclick = async function() {
                this.style.opacity = '0.5';
                this.style.pointerEvents = 'none';
                await verifyProof('image-solved');
            };
            // Start countdown
            const countdownEl = document.getElementById('countdown');
            updateCountdown(countdownEl);
            const countdownInterval = setInterval(() => {
                if (!updateCountdown(countdownEl)) clearInterval(countdownInterval);
            }, 1000);
            return;
        } else if (challenge.type === 'logic') {
            challengeUI.innerHTML = `
                <b>Logic Question:</b> What is 2 + 2? 
                <input id="logic-answer" type="text" size="4"> 
                <button id="logic-btn">Submit</button>
                <div id="countdown" style="font-size:12px;color:#666;margin-top:5px;"></div>
            `;
            document.getElementById('logic-btn').onclick = async function() {
                const answer = document.getElementById('logic-answer').value;
                if (answer.trim() === '4') {
                    this.disabled = true;
                    await verifyProof('logic-4');
                } else {
                    alert('Try again!');
                }
            };
            // Start countdown
            const countdownEl = document.getElementById('countdown');
            updateCountdown(countdownEl);
            const countdownInterval = setInterval(() => {
                if (!updateCountdown(countdownEl)) clearInterval(countdownInterval);
            }, 1000);
            return;
        } else if (challenge.difficulty === 0) {
            challengeUI.innerHTML = '<b>Invisible Challenge:</b> (No action needed, verifying...)';
        } else {
            challengeUI.innerHTML = '<b>Proof-of-Work Challenge:</b> Solving...';
        }

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
            if (hasLeadingZeroBits(hashArray, difficulty)) {
                console.log('collectFingerprint: Computed proof: ' + proof);
                break;
            }
            if (i === maxIterations - 1) {
                throw new Error('Failed to compute valid proof within iteration limit');
            }
        }
        await verifyProof(proof);

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