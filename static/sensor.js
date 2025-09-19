(function () {
  // Get the nonce from the script tag
  const s = document.currentScript;
  const nonce = s ? s.getAttribute("data-nonce") : "";

  // Payload to collect user telemetry
  const payload = {
    nonce: nonce,
    moves: [],
    scrolls: [],
    accel: [],
    canvas: ""
  };

  // Mouse movement tracking
  window.addEventListener("mousemove", function(e) {
    payload.moves.push({ x: e.clientX, y: e.clientY, t: Date.now() });
    if (payload.moves.length > 200) payload.moves.shift();
  }, { passive: true });

  // Scroll tracking
  window.addEventListener("scroll", function() {
    payload.scrolls.push({ y: window.scrollY || window.pageYOffset, t: Date.now() });
    if (payload.scrolls.length > 200) payload.scrolls.shift();
  }, { passive: true });

  // Device motion tracking (for mobile)
  if (window.DeviceMotionEvent) {
    window.addEventListener("devicemotion", function(e) {
      if (e.accelerationIncludingGravity) {
        const a = e.accelerationIncludingGravity;
        payload.accel.push({ x: a.x || 0, y: a.y || 0, z: a.z || 0, t: Date.now() });
        if (payload.accel.length > 200) payload.accel.shift();
      }
    }, { passive: true });
  }

  // Canvas fingerprinting
  (function () {
    try {
      const c = document.createElement("canvas");
      c.width = 200; c.height = 50;
      const ctx = c.getContext("2d");
      ctx.textBaseline = "top";
      ctx.font = "14px Arial";
      ctx.fillText("janus-proof", 2, 2);
      payload.canvas = c.toDataURL().slice(-200);
    } catch (e) {
      console.error("Canvas fingerprint error:", e);
    }
  })();

(function () {
    const s = document.currentScript;
    const nonce = s ? s.getAttribute("data-nonce") : "";

    const payload = {
        nonce: nonce,
        moves: [],
        scrolls: [],
        accel: [],
        canvas: ""
    };

    // telemetry sending function
    async function sendTelemetry(data) {
        try {
            const response = await fetch('/telemetry', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
    
            if (!response.ok) {
                const text = await response.text();
                console.error('Telemetry error:', text);
                return;
            }

            const result = await response.json(); // now safe
            console.log('Telemetry success:', result);

        } catch (err) {
            console.error('Telemetry fetch failed:', err);
        }
    }

    // Send payload every 5 seconds
    setInterval(() => sendTelemetry(payload), 5000);
});
})();
