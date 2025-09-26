// web/static/sensor.js
(function() {
    // --- Initial Verification (This now runs only if no session exists) ---
    function runInitialVerification() { /* ... (This function is the same as the last correct version) */ }

    // --- Continuous Monitoring Logic ---
    function startContinuousMonitoring() {
        let hasScrolled = false;
        let mouseBuffer = [];
        let bufferTimeout = null;

        // Function to send buffered mouse data
        function sendMouseActivity() {
            if (mouseBuffer.length > 0) {
                console.log(`JANUS SENSOR: Sending batch of ${mouseBuffer.length} mouse movements.`);
                fetch('/janus-activity', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' }, // Cookies are sent automatically
                    body: JSON.stringify({
                        activity: 'mousemove',
                        mouseSignature: mouseBuffer
                    })
                });
                mouseBuffer = []; // Clear the buffer
            }
        }

        function onMouseMove(event) {
            mouseBuffer.push({
                x: event.clientX,
                y: event.clientY,
                t: Date.now()
            });

            // If the buffer is full, send it immediately.
            if (mouseBuffer.length >= 20) {
                clearTimeout(bufferTimeout);
                sendMouseActivity();
            } else {
                // Otherwise, wait 2 seconds after the last movement to send.
                clearTimeout(bufferTimeout);
                bufferTimeout = setTimeout(sendMouseActivity, 2000);
            }
        }

        function onScroll() {
            if (!hasScrolled) {
                hasScrolled = true;
                console.log("JANUS SENSOR: Scroll detected, sending activity report.");
                fetch('/janus-activity', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ activity: 'scroll' })
                });
                window.removeEventListener('scroll', onScroll);
            }
        }

        // Add the listeners
        window.addEventListener('scroll', onScroll);
        // Only monitor mouse movement on desktops
        if (getDeviceType() === 'desktop') {
            document.addEventListener('mousemove', onMouseMove);
        }
    }
    
    // Helper function needed by monitoring
    function getDeviceType() { /* ... (same as before) */ }

    // --- Run Everything ---
    runInitialVerification();
    startContinuousMonitoring();
})();