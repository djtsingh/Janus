// example/server.js
// Simple Express backend for Janus testing.
// - Serves index + many pages
// - Serves OG preview route
// - Serves static assets from /public

const express = require("express");
const path = require("path");
const app = express();
const PORT = 3000;

// Serve static public files
app.use("/public", express.static(path.join(__dirname, "public")));

// Home page (long content for scroll detection)
app.get("/", (req, res) => {
  const longContent = Array.from({ length: 40 })
    .map((_, i) => `<p>Paragraph ${i+1}: Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p>`)
    .join("\n");

  res.send(`<!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>Janus Backend - Home</title>
      <meta property="og:title" content="Janus Backend Home" />
      <meta property="og:description" content="Test backend for Janus proxy." />
    </head>
    <body>
      <h1>Janus Backend Home</h1>
      <nav>
        <a href="/page/1">Page 1</a> |
        <a href="/page/2">Page 2</a> |
        <a href="/page/3">Page 3</a> |
        <a href="/og">OG Preview</a>
      </nav>
      ${longContent}
      <script src="/public/example.js"></script>
    </body>
  </html>`);
});

// Several internal pages to simulate navigation (deep links)
app.get("/page/:id", (req, res) => {
  const id = req.params.id;
  res.send(`<!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>Janus Backend - Page ${id}</title>
    </head>
    <body>
      <h1>Page ${id}</h1>
      <p>This is internal page ${id}. Use navigation to test Janus zombie detection.</p>
      <a href="/">Back home</a>
      <script>console.log("page ${id} loaded")</script>
    </body>
  </html>`);
});

// OG preview route (simulate social crawler)
app.get("/og", (req, res) => {
  res.send(`<!doctype html>
  <html>
    <head>
      <meta charset="utf-8" />
      <title>OG Preview</title>
      <meta property="og:title" content="Janus OG Preview" />
      <meta property="og:description" content="Open Graph preview served by backend." />
      <meta property="og:image" content="https://example.com/og-image.png" />
    </head>
    <body>
      <h1>Open Graph Preview Page</h1>
      <p>This page is for OG preview testing.</p>
    </body>
  </html>`);
});

// Simple API endpoint to echo telemetry (optional)
app.use(express.json());
app.post("/echo", (req, res) => {
  res.json({ ok: true, received: req.body });
});

app.listen(PORT, () => {
  console.log(`Backend server listening on http://localhost:${PORT}`);
});
