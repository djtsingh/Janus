async function getFingerprint() {
    try {
        // Simulate fingerprint collection (replace with actual canvas/WebGL logic)
        const fingerprint = {
            canvasHash: "test-canvas-" + Date.now(),
            webglRenderer: "none",
            webglVendor: "none",
            screen: { width: window.screen.width, height: window.screen.height },
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
            jsEnabled: true,
            isMobile: /Mobi|Android/i.test(navigator.userAgent)
        };
        console.log("Generated fingerprint:", fingerprint);
        return fingerprint;
    } catch (err) {
        console.error("Error generating fingerprint:", err);
        throw err;
    }
}

async function fetchChallenge(isMobile) {
    const url = isMobile ? "/janus/mobile-challenge" : "/janus/challenge";
    console.log("Fetching challenge from:", url);
    const response = await fetch(url);
    if (!response.ok) {
        console.error(`Failed to fetch challenge from ${url}: ${response.status} ${response.statusText}`);
        throw new Error(`Challenge fetch failed: ${response.status}`);
    }
    const challenge = await response.json();
    console.log("Received challenge:", challenge);
    return challenge;
}

async function computeProof(challenge) {
    console.log("Computing proof for challenge:", challenge);
    // Compute SHA-256 hash matching server-side logic
    const input = challenge.nonce + challenge.seed + challenge.iterations.toString();
    const msgBuffer = new TextEncoder().encode(input);
    const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer);
    const hashArray = Array.from(new Uint8Array(hashBuffer));
    const hashBase64 = btoa(String.fromCharCode(...hashArray));
    console.log("Computed proof:", hashBase64);
    return hashBase64;
}

async function sendFingerprint(attempt = 1, maxAttempts = 3) {
    try {
        const fingerprint = await getFingerprint();
        console.log(`Attempt ${attempt}: Sending fingerprint to /janus/fingerprint`, fingerprint);
        const fpResponse = await fetch("/janus/fingerprint", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(fingerprint),
        });
        if (!fpResponse.ok) {
            console.error(`Attempt ${attempt}: Fingerprint submission failed: ${fpResponse.status} ${fpResponse.statusText}`);
            if (attempt < maxAttempts) {
                console.log(`Retrying fingerprint submission (attempt ${attempt + 1})...`);
                await new Promise(resolve => setTimeout(resolve, 1000));
                return sendFingerprint(attempt + 1, maxAttempts);
            }
            throw new Error(`Fingerprint submission failed after ${maxAttempts} attempts: ${fpResponse.status}`);
        }
        console.log("Fingerprint submitted successfully");
        const challenge = await fetchChallenge(fingerprint.isMobile);
        const proof = await computeProof(challenge);
        console.log("Sending verification with proof:", proof, "nonce:", challenge.nonce);
        const verifyResponse = await fetch("/janus/verify", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ proof, nonce: challenge.nonce }),
        });
        if (!verifyResponse.ok) {
            console.error("Verification failed:", verifyResponse.status, verifyResponse.statusText);
            throw new Error("Verification failed");
        }
        console.log("Verification successful, reloading...");
        window.location.reload();
    } catch (err) {
        console.error("Error in fingerprint/challenge flow:", err);
    }
}

console.log("Starting sensor.js");
sendFingerprint();